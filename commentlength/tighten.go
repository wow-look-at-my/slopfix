// tighten.go shortens a comment by rewriting it, before anything is cut.
//
// Cutting is the blunt instrument: it removes a whole thought. Most over-long
// comments are not over-long by a thought, they are padded by words that carry
// nothing, and reflowed they fit. So the repair tries this before cutting, and
// cuts only what tightening cannot save.
package commentlength

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/english"
)

// danglingSpace matches the space a deletion leaves before punctuation that
// CLOSES something. A period with a word against its right side opens a file
// name or an extension, and closing the gap there welds it to the word before
// it: `the .gitmodules parser` became `the.gitmodules parser`.
var danglingSpace = regexp.MustCompile(`\s+([.,])(\s|$)`)

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

	// A paragraph break is structure, so reflow each paragraph on its own and
	var out []string
	rewrites := 0
	changed := false
	for _, para := range paragraphs(text) {
		if para.blank {
			out = append(out, indent+marker)
			continue
		}
		body := strings.Join(para.lines, " ")
		short, took := english.FixN(body, english.Comment)
		rewrites += took
		if short != body {
			changed = true
		}
		// A paragraph the table emptied is gone: what is left is punctuation
		// standing where a sentence was.
		if !hasWord(short) {
			continue
		}
		out = append(out, reflow(short, indent, marker, wrapWidth)...)
	}
	if !changed && len(out) >= len(text) {
		return text, 0, false
	}
	return out, rewrites, true
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

// Tighten rewrites a comment block shorter, reflowing its prose onto the
// block's own marker and width. It answers how many rewrites that took, and
// false when nothing it does makes the block smaller.
func Tighten(text []string) ([]string, int, bool) { return tighten(text) }

// wrapWidth is the column a reflowed comment wraps at, marker included.
const wrapWidth = 78
<<<<<<< HEAD
=======

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
	s = danglingSpace.ReplaceAllString(s, "${1}${2}")
	return capitalise(original, s)
}

// replaceWord swaps a whole word or phrase, case-insensitively, leaving a
// longer word that merely contains it alone.
func replaceWord(s, word, with string) string {
	lower := strings.ToLower(s)
	target := strings.ToLower(word)
	var b strings.Builder
	for i := 0; i < len(s); {
		j := strings.Index(lower[i:], target)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		at := i + j
		end := at + len(target)
		if !wordBoundary(s, at, end) {
			b.WriteString(s[i : at+1])
			i = at + 1
			continue
		}
		b.WriteString(s[i:at])
		b.WriteString(with)
		i = end
	}
	return b.String()
}

// wordBoundary reports whether s[at:end] stands as its own word.
func wordBoundary(s string, at, end int) bool {
	if at > 0 && isWordByte(s[at-1]) {
		return false
	}
	if end < len(s) && isWordByte(s[end]) {
		return false
	}
	return true
}

func isWordByte(b byte) bool {
	return b == '_' || b == '-' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// capitalise restores the opening capital a leading deletion can remove.
func capitalise(original, s string) string {
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
>>>>>>> origin/master
