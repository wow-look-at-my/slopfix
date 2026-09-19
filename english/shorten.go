// shorten.go applies the table. It is the whole of what a repair does to
// words, and it needs no parse: a caller hands it prose and gets prose back.
package english

import "strings"

// Surface names where prose is being read, which decides the entries that
// apply to it.
const (
	Comment = "comment"
	Message = "message"
)

// Fix rewrites prose with every table entry that names the surface.
func Fix(s, surface string) string {
	out, _ := FixN(s, surface)
	return out
}

// FixN is Fix, and the number of rewrites it took. A caller hands that count
// back to whoever wrote the prose, so the repair is visible rather than silent.
func FixN(s, surface string) (string, int) {
	original := s
	n := 0
	// Rewrites go before drops: a phrase like "in order to" would otherwise lose
	// its middle to a <drop> and stop matching as a phrase at all.
	for _, r := range Rewrites() {
		if AppliesTo(r.Where, surface) {
			var took int
			s, took = replaceWordN(s, r.From, r.To)
			n += took
		}
	}
	for _, d := range Drops() {
		if AppliesTo(d.Where, surface) {
			var took int
			s, took = replaceWordN(s, d.Word, "")
			n += took
		}
	}
	// Patterns last: they carry a shape rather than a phrase, and a shape must
	// see the text a word swap has already settled.
	for _, p := range Patterns() {
		if AppliesTo(p.Where, surface) {
			n += len(p.re.FindAllString(s, -1))
			s = p.Apply(s)
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, " .", ".")
	return capitalise(original, s), n
}

// ReplaceWord swaps a whole word or phrase, case-insensitively, leaving a
// longer word that merely contains it alone.
func ReplaceWord(s, word, with string) string {
	out, _ := replaceWordN(s, word, with)
	return out
}

func replaceWordN(s, word, with string) (string, int) {
	lower := strings.ToLower(s)
	target := strings.ToLower(word)
	took := 0
	var b strings.Builder
	for i := 0; i < len(s); {
		j := strings.Index(lower[i:], target)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		at := i + j
		end := at + len(target)
		if !WordBoundary(s, at, end) {
			b.WriteString(s[i : at+1])
			i = at + 1
			continue
		}
		b.WriteString(s[i:at])
		b.WriteString(with)
		took++
		i = end
	}
	return b.String(), took
}

// ContainsWord reports a whole-word occurrence, so `hack` never matches
// `hackney` and `for now` never matches `for nowhere`.
func ContainsWord(s, word string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], word)
		if j < 0 {
			return false
		}
		at := i + j
		if WordBoundary(s, at, at+len(word)) {
			return true
		}
		i = at + 1
	}
}

// WordBoundary reports whether s[at:end] stands as its own word.
func WordBoundary(s string, at, end int) bool {
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
