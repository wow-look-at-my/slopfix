// Package edit is how a repair changes a file: as byte-range edits a parser
// vouches for, never as a rewritten copy of the text.
//
// The mechanics live here. The parser that owns the file decides which bytes
// an edit may touch, and what must hold once it lands. That parser is the
// syntax tree for source, and the CommonMark tree for a document. Gate runs both checks.
package edit

import (
	"sort"
	"strings"
	"sync"
	"unsafe"
)

// Edit replaces the bytes from Start up to End with Text. The offsets count
// into the source the edit was made from.
type Edit struct {
	Start int
	End   int
	Text  string
	// Cut quotes what the edit takes out, for the report. It lands only with the edit.
	Cut []string
}

// Scope bounds where an edit may land. The zero Scope admits the whole file.
type Scope struct {
	Start, End int
	Bounded    bool
}

// Within is a Scope over the bytes from start up to end.
func Within(start, end int) Scope { return Scope{Start: start, End: end, Bounded: true} }

// Nowhere is a Scope that admits no edit, for a caller that wants the findings and no repair.
func Nowhere() Scope { return Scope{Start: 1, End: 0, Bounded: true} }

// Admits reports whether the edit lies inside the scope.
func (s Scope) Admits(e Edit) bool {
	return !s.Bounded || (e.Start >= s.Start && e.End <= s.End)
}

// Shift moves the scope over edits that landed, so it bounds the same text in
// the new source.
func (s Scope) Shift(applied []Edit) Scope {
	if !s.Bounded {
		return s
	}
	out := s
	for _, e := range applied {
		delta := len(e.Text) - (e.End - e.Start)
		if e.End <= s.Start {
			out.Start += delta
		}
		if e.End <= s.End {
			out.End += delta
		}
	}
	return out
}

// Refused is an edit a gate would not write, and the reason.
type Refused struct {
	Edit   Edit
	Reason string
}

// Result is what a gate wrote.
type Result struct {
	// Text is the source after every edit that passed the gate.
	Text string
	// Scope bounds, in Text, what the caller's Scope bounded.
	Scope Scope
	// Applied names each edit that landed.
	Applied []Edit
	// Refused names each edit that did not land.
	Refused []Refused
}

// Cuts quotes what every landed edit took out.
func (r Result) Cuts() []string {
	var out []string
	for _, e := range r.Applied {
		out = append(out, e.Cut...)
	}
	return out
}

// Then adds a later pass to r: its text and scope replace r's, and its edits
// join r's.
func (r Result) Then(next Result) Result {
	next.Applied = append(append([]Edit{}, r.Applied...), next.Applied...)
	next.Refused = append(append([]Refused{}, r.Refused...), next.Refused...)
	return next
}

// Unchanged is the Result of a pass that wrote nothing.
func Unchanged(src string, scope Scope) Result { return Result{Text: src, Scope: scope} }

// Gate writes the edits into src. An edit outside the scope is dropped without
// a finding, because the caller asked for no change there. reach answers why an
// edit touches bytes it must not, or "". holds reports whether a candidate text
// keeps what the parser demands. A batch that fails holds is retried an edit at
// a time, so a single bad rewrite does not cost the file every good one.
func Gate(src string, edits []Edit, scope Scope, reach func(Edit) string, holds func(string) bool) Result {
	out := Unchanged(src, scope)
	var candidates []Edit
	for _, e := range sorted(edits) {
		if !scope.Admits(e) {
			continue
		}
		if e.Start < 0 || e.End > len(src) || e.Start > e.End {
			out.Refused = append(out.Refused, Refused{Edit: e, Reason: "its bytes are outside the source"})
			continue
		}
		if reason := reach(e); reason != "" {
			out.Refused = append(out.Refused, Refused{Edit: e, Reason: reason})
			continue
		}
		if n := len(candidates); n > 0 && e.Start < candidates[n-1].End {
			out.Refused = append(out.Refused, Refused{Edit: e, Reason: "it overlaps an earlier edit"})
			continue
		}
		candidates = append(candidates, e)
	}
	if len(candidates) == 0 {
		return out
	}
	if text := Splice(src, candidates); holds(text) {
		out.Text, out.Scope, out.Applied = text, scope.Shift(candidates), candidates
		return out
	}
	// Halve the batch until each part holds or is one refused edit.
	var landed []Edit
	var settle func(group []Edit)
	settle = func(group []Edit) {
		if len(group) == 0 {
			return
		}
		trial := append(append([]Edit{}, landed...), group...)
		if holds(Splice(src, trial)) {
			landed = trial
			return
		}
		if len(group) == 1 {
			out.Refused = append(out.Refused, Refused{Edit: group[0], Reason: "the rewrite changes what the parser reads outside it"})
			return
		}
		settle(group[:len(group)/2])
		settle(group[len(group)/2:])
	}
	settle(candidates[:len(candidates)/2])
	settle(candidates[len(candidates)/2:])
	out.Text, out.Scope, out.Applied = Splice(src, landed), scope.Shift(landed), landed
	return out
}

// sorted orders edits by where they start.
func sorted(edits []Edit) []Edit {
	out := append([]Edit{}, edits...)
	sort.SliceStable(out, func(a, b int) bool { return out[a].Start < out[b].Start })
	return out
}

// Splice writes sorted, disjoint edits into src.
func Splice(src string, edits []Edit) string {
	var b strings.Builder
	at := 0
	for _, e := range edits {
		b.WriteString(src[at:e.Start])
		b.WriteString(e.Text)
		at = e.End
	}
	b.WriteString(src[at:])
	return b.String()
}

// Lines is an Edit over whole lines, counted from zero, of src. The first col
// bytes of line from stay as written. With no lines and a col of zero, the lines
// go, line ends and all.
func Lines(src string, from, to, col int, lines []string) Edit {
	starts := lineStarts(src)
	if from < 0 || to >= len(starts) || from > to {
		return Edit{Start: -1, End: -1}
	}
	start := starts[from] + col
	end := lineEnd(src, starts, to)
	if len(lines) == 0 && col == 0 {
		switch {
		case to+1 < len(starts):
			end = starts[to+1]
		case start > 0:
			// The last line has no line end of its own, so the one before it goes.
			start--
		}
		return Edit{Start: start, End: end}
	}
	return Edit{Start: start, End: end, Text: strings.Join(lines, "\n")}
}

// lineStarts answers the byte each line opens at. A repair asks it once for each
// edit on the same text, so it remembers recent texts. Callers must not change the slice.
func lineStarts(src string) []int {
	key := lineKey{data: unsafe.StringData(src), n: len(src)}
	lineCache.mu.Lock()
	entry, ok := lineCache.byKey[key]
	lineCache.mu.Unlock()
	if ok {
		return entry.starts
	}
	starts := scanLines(src)
	lineCache.mu.Lock()
	defer lineCache.mu.Unlock()
	if _, ok := lineCache.byKey[key]; !ok {
		if len(lineCache.order) == lineCacheSize {
			delete(lineCache.byKey, lineCache.order[0])
			lineCache.order = lineCache.order[1:]
		}
		lineCache.order = append(lineCache.order, key)
		// The entry holds src, so its bytes stay put and no other text takes the key.
		lineCache.byKey[key] = lineEntry{src: src, starts: starts}
	}
	return starts
}

// lineKey names a text by where its bytes sit. A string never changes in place.
type lineKey struct {
	data *byte
	n    int
}

type lineEntry struct {
	src    string
	starts []int
}

// lineCacheSize bounds the remembered texts: a few for each worker of a tree walk.
const lineCacheSize = 64

var lineCache = struct {
	mu    sync.Mutex
	order []lineKey
	byKey map[lineKey]lineEntry
}{byKey: map[lineKey]lineEntry{}}

// scanLines finds the byte each line opens at.
func scanLines(src string) []int {
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineEnd answers the byte a line's end sits at, or the end of src.
func lineEnd(src string, starts []int, line int) int {
	if line+1 < len(starts) {
		return starts[line+1] - 1
	}
	return len(src)
}

// Offset answers the byte a line and column name, or a negative for a line src
// does not have.
func Offset(src string, line, col int) int {
	starts := lineStarts(src)
	if line < 0 || line >= len(starts) {
		return -1
	}
	return starts[line] + col
}
