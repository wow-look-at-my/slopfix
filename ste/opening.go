package ste

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// capitalizeOpening capitalizes a lower-case letter at the start, and leaves anything else.
func capitalizeOpening(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	if !unicode.IsLower(first) {
		return s
	}
	word := s
	if end := strings.IndexAny(s, " \t"); end >= 0 {
		word = s[:end]
	}
	// An identifier keeps the case its code spells it with.
	if strings.ContainsFunc(word[width:], unicode.IsUpper) || strings.ContainsAny(word, "_.") {
		return s
	}
	return string(unicode.ToUpper(first)) + s[width:]
}
