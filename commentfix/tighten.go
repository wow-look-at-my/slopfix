// tighten.go shortens a comment by rewriting it, before anything is cut.
//
// Cutting is the blunt instrument: it removes a whole thought. Most over-long
// comments are not over-long by a thought, they are padded by words that carry
// nothing, and reflowed they fit. So the repair tries this before cutting, and
// cuts only what tightening cannot save.
package commentfix

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/english"
)

// tighten rewrites a comment run: it drops filler, applies the shorter phrasing,
// and reflows the prose to the block's own marker and width.
//
// It returns false when nothing changed, so the caller knows tightening bought
// nothing and it is time to cut.
func tighten(text []string) ([]string, int, bool) {
	if out, n, ok := starBlock(text); ok {
		return out, n, true
	}
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, 0, false
	}

	// A paragraph break is structure, so reflow each paragraph on its own and put the breaks back between them.
	var out []string
	rewrites := 0
	changed, emptied := false, false
	for _, para := range paragraphs(text) {
		if para.blank {
			out = append(out, indent+marker)
			continue
		}
		if para.verbatim {
			// A code block is a table.
			out = append(out, para.raw...)
			continue
		}
		if para.list() {
			for i, line := range para.raw {
				short, took := english.FixN(para.lines[i], english.Comment)
				rewrites += took
				if short != para.lines[i] {
					changed = true
				}
				if !hasWord(short) {
					continue
				}
				// The prefix keeps an item's hanging indent.
				prefix, _, found := strings.Cut(line, para.lines[i])
				if !found {
					out = append(out, line)
					continue
				}
				out = append(out, prefix+short)
			}
			continue
		}
		body := strings.Join(para.lines, " ")
		short, took := english.FixN(body, english.Comment)
		rewrites += took
		if short != body {
			changed = true
		}
		// A paragraph the table emptied is gone.
		if !hasWord(short) {
			emptied = true
			continue
		}
		out = append(out, reflow(short, indent, marker, wrapWidth)...)
	}
	if emptied {
		out = tidyBreaks(out, indent+marker)
	}
	if !changed && len(out) >= len(text) {
		return text, 0, false
	}
	return out, rewrites, true
}

// tidyBreaks drops the paragraph breaks an emptied paragraph leaves: one at
// either end of the block, and one beside another.
func tidyBreaks(lines []string, blank string) []string {
	var out []string
	for _, line := range lines {
		if line == blank && (len(out) == 0 || out[len(out)-1] == blank) {
			continue
		}
		out = append(out, line)
	}
	for len(out) > 0 && out[len(out)-1] == blank {
		out = out[:len(out)-1]
	}
	return out
}

// starBlock rewrites a /* ... */ run. The delimiters carry no prose, so the
// where they are. A run the table empties goes entirely: a pair of delimiters
// around nothing is not a comment.
func starBlock(text []string) ([]string, int, bool) {
	if len(text) < 3 {
		return nil, 0, false
	}
	open, shut := strings.TrimSpace(text[0]), strings.TrimSpace(text[len(text)-1])
	if open != "/*" || shut != "*/" {
		return nil, 0, false
	}
	indent := text[0][:len(text[0])-len(strings.TrimLeft(text[0], " \t"))]
	short, n := english.FixN(strings.Join(text[1:len(text)-1], " "), english.Comment)
	if n == 0 {
		return nil, 0, false
	}
	if !hasWord(short) {
		return nil, n, true
	}
	out := []string{text[0]}
	for _, line := range reflow(short, indent, "", wrapWidth) {
		out = append(out, indent+strings.TrimSpace(line))
	}
	return append(out, text[len(text)-1]), n, true
}

// hasWord reports whether any letter or digit survives, so a comment reduced to
// its punctuation is recognised as empty.
func hasWord(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// Tighten rewrites a comment block shorter, reflowing its prose onto the block's own marker and width. It answers how
func Tighten(text []string) ([]string, int, bool) { return tighten(text) }

// wrapWidth is the column a reflowed comment wraps at, marker included.
const wrapWidth = 78

// shorten rewrites a single comment sentence.
func shorten(s string) string { return english.Fix(s, english.Comment) }

// Deslop rewrites a rendered message, applying every entry that names the message surface.
func Deslop(s string) string { return english.Fix(s, english.Message) }
