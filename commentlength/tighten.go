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

// filler is a word or phrase that survives its own deletion. Each entry is
// checked whole, lowercased, and only between word boundaries.
//
// The list is deliberately short and deliberately boring. A word that changes
// meaning in any context does not belong here: a wrong deletion is worse than
// a comment left long, because nobody reviews what a repair applied.
var filler = []string{
	"actually", "basically", "essentially", "fundamentally", "simply",
	"obviously", "clearly", "of course", "in fact", "indeed",
	"really", "very", "quite", "rather", "somewhat", "fairly",
	"just", "note that", "it is worth noting that", "please note that",
	"in order to", "for the purposes of", "with respect to",
	"at the end of the day", "needless to say",
}

// replacement rewrites a phrase to a shorter one that means the same thing.
var replacement = map[string]string{
	"in order to":           "to",
	"due to the fact that":  "because",
	"in the event that":     "if",
	"for the purpose of":    "for",
	"a large number of":     "many",
	"at this point in time": "now",
	"is able to":            "can",
	"has the ability to":    "can",
	"make use of":           "use",
	"take into account":     "consider",
}

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
	// keep the blank markers between them.
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
func shorten(s string) string {
	for phrase, with := range replacement {
		s = replaceWord(s, phrase, with)
	}
	for _, word := range filler {
		s = replaceWord(s, word, "")
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
