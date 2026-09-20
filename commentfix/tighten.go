// tighten.go shortens a comment by rewriting it, before anything is cut.
//
// Cutting is the blunt instrument: it removes a whole thought. Most over-long
// comments are not over-long by a thought, they are padded by words that carry
// nothing, and reflowed they fit. So the repair tries this before cutting, and
// cuts only what tightening cannot save.
package commentfix

import (
	"strings"
)

// tighten rewrites a comment run: it drops filler, applies the shorter phrasing,
// and reflows the prose to the block's own marker and width.
//
// It returns false when nothing changed, so the caller knows tightening bought
// nothing and it is time to cut.
func tighten(text []string) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, false
	}

	// A paragraph break is structure, so reflow each paragraph on its own and
	var out []string
	changed := false
	for _, para := range paragraphs(text) {
		if para.blank {
			out = append(out, indent+marker)
			continue
		}
		body := strings.Join(para.lines, " ")
		short := shorten(body)
		if short != body {
			changed = true
		}
		out = append(out, reflow(short, indent, marker, wrapWidth)...)
	}
	if !changed && len(out) >= len(text) {
		return text, false
	}
	return out, true
}

// wrapWidth is the column a reflowed comment wraps at, marker included.
const wrapWidth = 78

// shorten drops filler and applies the shorter phrasing, then tidies the
func shorten(s string) string { return shortenFor(s, "comment") }

// Deslop rewrites a rendered message, applying every entry that names the
func Deslop(s string) string { return shortenFor(s, "message") }

// shortenFor applies the table entries that name a surface.
func shortenFor(s, surface string) string {
	original := s
	// Rewrites go before drops: a phrase like "in order to" would otherwise lose its
	// middle to a <drop> and stop matching as a phrase at all.
	for _, r := range english.Rewrites {
		if appliesTo(r.Where, surface) {
			s = replaceWord(s, r.From, r.To)
		}
	}
	for _, d := range english.Drops {
		if appliesTo(d.Where, surface) {
			s = replaceWord(s, d.Word, "")
		}
	}
	// Patterns last: they carry a shape rather than a phrase, and a shape must
	// see the text a word swap has already settled.
	for _, p := range english.Patterns {
		if appliesTo(p.Where, surface) {
			s = p.Replace(s)
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, " .", ".")
	return capitaliseAfterCut(original, s)
}

// Word replacement and its boundary test live in table.go. This copy read a
// name's dot and slash as a boundary, so `strings.Only` matched `only`.

// capitaliseAfterCut restores the opening capital a leading deletion removes.
//
// It differs from table.go's capitalise, which takes the case of the line it
// repaired. Here the opening word is GONE, so the source line's case belongs to
func capitaliseAfterCut(original, s string) string {
	if s == "" || sameFirstWord(original, s) {
		return s
	}
	if c := s[0]; c >= 'a' && c <= 'z' {
		return string(c-32) + s[1:]
	}
	return s
}

// sameFirstWord reports whether the repair left the opening word in place.
func sameFirstWord(original, s string) bool {
	before, after := strings.Fields(original), strings.Fields(s)
	if len(before) == 0 || len(after) == 0 {
		return false
	}
	return before[0] == after[0]
}
