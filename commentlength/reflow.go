// reflow.go is the shape half of tightening: what a comment's marker and indent
// are, where its paragraphs break, and how prose wraps back onto lines.
package commentlength

import (
	"strings"
)

// paragraph is a run of comment lines, or the blank marker between runs.
type paragraph struct {
	lines []string
	blank bool
}

// commentShape reads the indent and marker a block uses, from its opening line.
// It reports false for a block whose lines disagree, because rewriting any of
// those would change more than the prose.
func commentShape(text []string) (marker, indent string, ok bool) {
	if len(text) == 0 {
		return "", "", false
	}
	first := text[0]
	trimmed := strings.TrimLeft(first, " \t")
	indent = first[:len(first)-len(trimmed)]
	for _, m := range []string{"///", "//", "#"} {
		if strings.HasPrefix(trimmed, m) {
			marker = m
			break
		}
	}
	if marker == "" {
		return "", "", false
	}
	for _, line := range text {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, marker) {
			continue
		}
		return "", "", false
	}
	return marker, indent, true
}

// paragraphs splits a block on its blank comment lines, keeping the breaks.
func paragraphs(text []string) []paragraph {
	var out []paragraph
	var run []string
	flush := func() {
		if len(run) > 0 {
			out = append(out, paragraph{lines: run})
			run = nil
		}
	}
	for _, line := range text {
		if isBlankComment(line) {
			flush()
			out = append(out, paragraph{blank: true})
			continue
		}
		out = append(out, paragraph{})
		out = out[:len(out)-1]
		run = append(run, stripMarker(line))
	}
	flush()
	return out
}

// stripMarker removes the indent and comment marker, leaving the prose.
func stripMarker(line string) string {
	t := strings.TrimSpace(line)
	for _, m := range []string{"///", "//", "#"} {
		if rest, found := strings.CutPrefix(t, m); found {
			return strings.TrimSpace(rest)
		}
	}
	return t
}

// reflow wraps prose back onto comment lines at the given width.
//
// A word longer than the width goes on its own line rather than being broken:
// a URL or an identifier split across lines stops being either.
func reflow(body, indent, marker string, width int) []string {
	words := strings.Fields(body)
	if len(words) == 0 {
		return []string{indent + marker}
	}
	prefix := indent + marker + " "
	var out []string
	line := prefix
	for _, w := range words {
		if line != prefix && len(line)+1+len(w) > width {
			out = append(out, strings.TrimRight(line, " "))
			line = prefix
		}
		if line != prefix {
			line += " "
		}
		line += w
	}
	return append(out, strings.TrimRight(line, " "))
}
