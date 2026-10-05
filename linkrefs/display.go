// display.go rewrites a streaming message so every reference the reader might
// open arrives clickable.
//
// This is the whole enforcement. There is no Stop hook and nothing is sent back
// to the model, because the token plus the checkout determine the URL, so the
// hook writes it. Asking the model to re-emit the message costs a round trip,
// and the reply explaining itself names the reference again and trips the
// guard again.
//
// displayContent is display-only, verified against the shipped bundle: the
// transcript and the model's next request are both fed from an array populated
// BEFORE this hook runs and never updated from its result. So this changes what
// the reader sees and nothing else.
package linkrefs

import (
	"slices"
	"strings"
)

// RewriteDelta returns the rendered form of a flush, and whether anything
// changed. A false return means print nothing, which leaves the CLI showing the
// original text.
func RewriteDelta(delta string, insideFence bool, res Resolver) (string, bool) {
	if delta == "" {
		return "", false
	}
	fence := insideFence
	changed := false
	lines := strings.Split(delta, "\n")

	// Every line to rewrite, found first, so the pull requests they name are asked about together.
	live := make([]bool, len(lines))
	var pulls []PullRef
	for i, line := range lines {
		if fenceMarker(line) != "" {
			fence = !fence
			continue
		}
		if fence || isQuoted(line) {
			continue
		}
		live[i] = true
		pulls = append(pulls, linePulls(line)...)
	}
	if p, ok := res.(prefetcher); ok && len(pulls) > 0 {
		p.Prefetch(pulls)
	}

	for i, line := range lines {
		if !live[i] {
			continue
		}
		rewritten, ok := rewriteLine(line, res)
		if !ok {
			continue
		}
		lines[i] = rewritten
		changed = true
	}

	if !changed {
		return "", false
	}
	return strings.Join(lines, "\n"), true
}

// splice is a replacement for the bytes [start,end) of a line.
type splice struct {
	start, end int
	text       string
}

// linePulls names every pull request a line refers to, linked or not.
func linePulls(line string) []PullRef {
	var out []PullRef
	for _, ref := range FindUnlinkedInLine(line) {
		if p, ok := pullRefOf(ref.Ref); ok {
			out = append(out, p)
		}
	}
	for _, l := range findLinks(line) {
		if repo, n, ok := IssueRef(l.url); ok {
			out = append(out, PullRef{repo, n})
		}
	}
	return out
}

// rewriteLine links every bare reference that resolves to a page, and puts a
// dot beside every link to a pull request, whoever wrote the link.
func rewriteLine(line string, res Resolver) (string, bool) {
	var edits []splice
	// The words after a finished dot belong to it, so they are not linked again.
	dotted := map[int]bool{}
	for _, l := range findLinks(line) {
		if isDot(strings.TrimSpace(l.text)) {
			dotted[l.end+1] = true
		}
	}
	for _, ref := range FindUnlinkedInLine(line) {
		if dotted[ref.Start] {
			continue
		}
		if link, ok := Linkify(ref.Ref, res); ok {
			edits = append(edits, splice{ref.Start, ref.End, link})
		}
	}
	for _, l := range findLinks(line) {
		if endsWithDot(line[:l.start]) {
			continue
		}
		if badged, ok := BadgeLink(line[l.start:l.end], l.text, l.url, res); ok {
			edits = append(edits, splice{l.start, l.end, badged})
		}
	}
	if len(edits) == 0 {
		return "", false
	}
	slices.SortFunc(edits, func(a, b splice) int { return a.start - b.start })
	out := line
	// Right to left, so an earlier offset stays valid after a later splice.
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		out = out[:e.start] + e.text + out[e.end:]
	}
	return out, true
}

// isQuoted reports the line shapes a message uses to quote rather than assert: a blockquote or indented code.
func isQuoted(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, ">") {
		return true
	}
	return strings.HasPrefix(line, "    ") && trimmed != ""
}

// EndsInsideFence reports whether text finishes inside a fenced code block. A
// flush cannot see the ``` that opened several lines earlier, so the state has
// to be carried across flushes.
func EndsInsideFence(text string) bool {
	fence := ""
	for _, line := range strings.Split(text, "\n") {
		marker := fenceMarker(line)
		if marker == "" {
			continue
		}
		if fence == "" {
			fence = marker
		} else if marker == fence {
			fence = ""
		}
	}
	return fence != ""
}
