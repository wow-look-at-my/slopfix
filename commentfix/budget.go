// budget.go brings a comment block under its line cap by rewriting it. It is
// the comment half of the STE budget: ste.FitToBudget does the prose work. This
// lays the result back onto the block's own marker. Nothing here deletes a
// sentence. A list line is never welded to its neighbour, so a block the
// rewrite cannot fit is left for the caller to cut.
package commentfix

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/ste"
)

// fitVolume rewrites a block to fit maxLines, before any cut. A plain prose
// paragraph is fitted to the characters the lines it already holds can carry,
// which the STE budget spends by tightening words. And restating clauses. A
// list or heading stands as written. It reports false when the rewrite cannot
// reach the cap.
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
		case listParagraph(para.lines):
			// A list item, a heading or an aligned row is one line by design,
			// so reflowing it would weld its items together.
			for _, line := range para.lines {
				out = append(out, prefix+line)
			}
			lines += len(para.lines)
		default:
			prose := strings.Join(para.lines, " ")
			short, _ := ste.FitToBudget(prose, len(para.lines)*width, func(s string) string {
				return english.Fix(s, english.Comment)
			})
			if short == prose {
				for _, line := range para.lines {
					out = append(out, prefix+line)
				}
				lines += len(para.lines)
				continue
			}
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

// listParagraph reports a paragraph holding a list item, a heading or an
// aligned row. Such a line carries its own shape, which reflow would destroy.
func listParagraph(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		switch trimmed[0] {
		case '*', '-', '+', '#', '@', '|', '>':
			return true
		}
		if unicode.IsDigit(rune(trimmed[0])) {
			for i := 0; i < len(trimmed); i++ {
				if trimmed[i] == '.' || trimmed[i] == ')' {
					return true
				}
				if !unicode.IsDigit(rune(trimmed[i])) {
					break
				}
			}
		}
	}
	return false
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
