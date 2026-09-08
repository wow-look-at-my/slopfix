// tighten.go shortens a comment by rewriting it, before anything is cut.
//
// Cutting is the blunt instrument: it removes a whole thought. Most over-long
// comments are not over-long by a thought, they are padded by words that carry
// nothing, and reflowed they fit. So the repair tries this first and only cuts
// what tightening cannot save.
package commentlength

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
// spacing the deletions leave behind.
func shorten(s string) string { return shortenFor(s, "comment") }

// Deslop rewrites one rendered message, applying every entry that names the
// message surface. It is the real-time half: a MessageDisplay hook hands it the
// delta as it streams and shows what comes back.
//
// It rewrites and never annotates. An annotation about a phrase the reader can
// already see is a second thing to read; replacing it costs the reader nothing.
func Deslop(s string) string { return shortenFor(s, "message") }

// shortenFor applies the table entries that name a surface.
func shortenFor(s, surface string) string {
	// Rewrites first: a phrase like "in order to" would otherwise lose its
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
			s = p.re.ReplaceAllString(s, p.To)
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, " .", ".")
	return capitalise(s)
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
func capitalise(s string) string {
	if s == "" {
		return s
	}
	if c := s[0]; c >= 'a' && c <= 'z' {
		return string(c-32) + s[1:]
	}
	return s
}
