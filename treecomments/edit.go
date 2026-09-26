// edit.go is the door a repair writes source through.
//
// An edit names bytes the tree calls comment. The gate refuses any byte it
// covers that is neither comment nor the blank around it. After the splice
// the source is parsed again. Every node that is not a comment must come back
// with the same type and the same text, in the same place in the tree. Text
// that closes a comment early, a newline that ends a line comment, or a
// marker that opens a new one changes that tree, so the edit that wrote it
// never lands.
package treecomments

import (
	"fmt"
	"sort"
	"strings"

	ts "github.com/wow-look-at-my/go-tree-sitter"
	"github.com/wow-look-at-my/slopfix/edit"
)

// Apply writes the edits into src through the gate. An edit outside the scope
// is dropped without a finding: the caller asked for no change there. An edit
// that reaches code, or that changes the code tree once written, is refused.
func Apply(filename, src string, edits []edit.Edit, scope edit.Scope) edit.Result {
	if len(edits) == 0 {
		return edit.Unchanged(src, scope)
	}
	refuseAll := func(reason string) edit.Result {
		out := edit.Unchanged(src, scope)
		for _, e := range edits {
			out.Refused = append(out.Refused, edit.Refused{Edit: e, Reason: reason})
		}
		return out
	}
	language := languageFor(filename)
	if language == nil {
		return refuseAll("no grammar reads " + filename)
	}
	root, ok := parseWith(language, src)
	if !ok {
		return refuseAll("the source does not parse")
	}
	spans := commentSpans(filename, src)
	want := shape(root, src)
	return edit.Gate(src, edits, scope,
		func(e edit.Edit) string { return reach(src, spans, e) },
		func(text string) bool { return sameShape(language, text, want) })
}

// span is where a comment node sits.
type span struct{ start, end int }

// commentSpans answers every comment a repair may write into. The interpreter
// line and a cgo preamble are left out, because a tool reads them as code.
func commentSpans(filename, src string) []span {
	var out []span
	for _, c := range Extract(filename, src) {
		out = append(out, span{start: c.Offset, end: c.Offset + len(c.Text)})
	}
	return out
}

// reach answers why an edit covers bytes that are not its own, or "".
func reach(src string, spans []span, e edit.Edit) string {
	touches := false
	for at := e.Start; at < e.End; at++ {
		if inside(spans, at) {
			touches = true
			continue
		}
		if !blank(src[at]) {
			return fmt.Sprintf("byte %d is code, not comment", at)
		}
	}
	// A pure insertion lands at a comment's edge or inside it.
	if e.Start == e.End {
		touches = inside(spans, e.Start) || (e.Start > 0 && inside(spans, e.Start-1))
	}
	if !touches {
		return "it touches no comment"
	}
	return ""
}

// inside reports whether the byte at is part of a comment.
func inside(spans []span, at int) bool {
	i := sort.Search(len(spans), func(i int) bool { return spans[i].end > at })
	return i < len(spans) && spans[i].start <= at
}

func blank(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

// shape renders every node that is not a comment: its depth, its type, and
// for a token its text. Whitespace is no node, so a comment edit that stays a
// comment leaves the shape as it was.
func shape(root ts.Node, src string) []string {
	var out []string
	var walk func(n ts.Node, depth int)
	walk = func(n ts.Node, depth int) {
		if strings.Contains(n.Type(), "comment") {
			return
		}
		entry := fmt.Sprintf("%d %s", depth, n.Type())
		if n.IsMissing() {
			entry += " missing"
		}
		count := n.ChildCount()
		if count == 0 {
			start, end := int(n.StartByte()), int(n.EndByte())
			if start >= 0 && end <= len(src) && start <= end {
				entry += " " + fmt.Sprintf("%q", src[start:end])
			}
		}
		out = append(out, entry)
		for i := uint32(0); i < count; i++ {
			walk(n.Child(i), depth+1)
		}
	}
	walk(root, 0)
	return out
}

// sameShape reports whether text parses to the code tree want describes.
func sameShape(language *ts.Language, text string, want []string) bool {
	root, ok := parseWith(language, text)
	if !ok {
		return false
	}
	got := shape(root, text)
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// parseWith parses src and answers the root.
func parseWith(language *ts.Language, src string) (ts.Node, bool) {
	parser := ts.NewParser()
	if !parser.SetLanguage(language) {
		return ts.Node{}, false
	}
	return parse(parser, src)
}

