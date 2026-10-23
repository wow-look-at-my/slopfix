// capitalizeOpening capitalizes a lower-case letter at the start, and leaves anything else.

package ste

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// capitalizeOpening capitalizes a lower-case letter at the start, and leaves anything else.
// A name such as foreignJobs keeps its case, because a capital makes a name nothing defines.
func capitalizeOpening(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	if !unicode.IsLower(first) || NameLike(s) {
		return s
	}
	return string(unicode.ToUpper(first)) + s[width:]
}

// NameLike reports an opening word that reads as an identifier: a capital or
// an underscore after its first letter, or a digit beside a letter.
func NameLike(s string) bool {
	word := s
	if end := strings.IndexFunc(s, unicode.IsSpace); end >= 0 {
		word = s[:end]
	}
	word = strings.TrimRight(word, ".,;:!?)\"'")
	var prev rune
	for i, r := range word {
		switch {
		case i > 0 && (unicode.IsUpper(r) || r == '_'):
			return true
		case unicode.IsDigit(r) && unicode.IsLetter(prev), unicode.IsLetter(r) && unicode.IsDigit(prev):
			return true
		}
		prev = r
	}
	return false
}
