// budget.go brings a comment block under its line cap by rewriting it. It is
// the comment half of the STE budget: ste.FitToBudget does the prose work. This
// lays the result back onto the block's own marker. Nothing here deletes a
// sentence, so a block that cannot fit is left for the caller to cut.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/ste"
)

// fitVolume rewrites a block to fit maxLines, before any cut. Each prose
// paragraph is fitted to the characters the lines it already holds can carry,
// which the STE budget spends by tightening words. And restating clauses. It
// reports false when the rewrite cannot reach the cap.
func fitVolume(text []string, maxLines int) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, false
	}
	prefix := indent + marker + " "
	width := wrapWidth - len(prefix)
	if width <= 0 {
		return text, false
	}
	var out []string
	lines := 0
	changed := false
	for _, para := range paragraphs(text) {
		switch {
		case para.blank:
			out = append(out, indent+marker)
			lines++
		case para.verbatim:
			out = append(out, para.raw...)
			lines += len(para.raw)
		default:
			prose := strings.Join(para.lines, " ")
			short, _ := ste.FitToBudget(prose, len(para.lines)*width, func(s string) string {
				return english.Fix(s, english.Comment)
			})
			rewritten := reflow(short, indent, marker, wrapWidth)
			if len(rewritten) < len(para.lines) {
				changed = true
			}
			out = append(out, rewritten...)
			lines += len(rewritten)
		}
	}
	// A blank marker line says nothing, so a block still over the cap gives its
	// blanks up before any prose does.
	if lines > maxLines {
		if kept, ok := dropBlankSeparators(out, maxLines); ok {
			return kept, true
		}
	}
	if !changed || lines > maxLines {
		return text, false
	}
	return out, true
}

// dropBlankSeparators drops every blank comment line of a block, and reports
// whether that alone reached the cap. It keeps every line that says anything.
func dropBlankSeparators(lines []string, maxLines int) ([]string, bool) {
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if isBlankComment(line) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) <= maxLines {
		return kept, true
	}
	return lines, false
}
