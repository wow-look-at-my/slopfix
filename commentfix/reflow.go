// reflow.go is the shape half of tightening: what a comment's marker and indent
// are, where its paragraphs break, and how prose wraps back onto lines.
package commentfix

import (
	"strings"
)

// paragraph is a run of comment lines, or the blank marker between runs.
//
// A verbatim paragraph carries the source lines rather than their prose,
// because a godoc code block is a table the reader reads by its columns.
type paragraph struct {
	lines    []string
	blank    bool
	verbatim bool
	raw      []string
}

// codeRow reports a line godoc renders verbatim: the prose after its marker
// opens with a tab, which is how a doc comment spells a code block.
func codeRow(line string) bool {
	t := strings.TrimLeft(line, " \t")
	for _, m := range []string{"///", "//", "#"} {
		if rest, found := strings.CutPrefix(t, m); found {
			return strings.HasPrefix(rest, "\t")
		}
	}
	return false
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
	var block []string
	flushBlock := func() {
		if len(block) > 0 {
			out = append(out, paragraph{verbatim: true, raw: block})
			block = nil
		}
	}
	for _, line := range text {
		if isBlankComment(line) {
			flush()
			flushBlock()
			out = append(out, paragraph{blank: true})
			continue
		}
		if codeRow(line) {
			flush()
			block = append(block, line)
			continue
		}
		flushBlock()
		run = append(run, stripMarker(line))
	}
	flushBlock()
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

// widen lays a block's prose out at the given width rather than the default.
//
// A paragraph break is structure, so each paragraph is laid out on its own.
// It reports false when the result is no shorter, which leaves the caller to
// cut instead.
func widen(text []string, width int) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, false
	}
	var out []string
	for _, para := range paragraphs(text) {
		if para.blank {
			out = append(out, indent+marker)
			continue
		}
		if para.verbatim {
			out = append(out, para.raw...)
			continue
		}
		out = append(out, reflow(strings.Join(para.lines, " "), indent, marker, width)...)
	}
	if len(out) >= len(text) {
		return text, false
	}
	return out, true
}
