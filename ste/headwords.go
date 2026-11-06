// listsVerbs reports a comma after the main verb of head.

package ste

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// listsVerbs reports a comma after the main verb of head. A verb group after
// the cut then continues a list of verb groups, as in "it reads, writes and
// holds".
func listsVerbs(head string) bool {
	s := syntax.Parse(strings.TrimRight(head, " ,"), nil)
	for _, c := range s.Clauses {
		if c.Depth != 0 || c.Verb == nil {
			continue
		}
		for i := c.Verb.Last + 1; i < len(s.Words); i++ {
			if s.Words[i].Text == "," {
				return true
			}
		}
		return false
	}
	return false
}

// lowerIdentifier reports a word that opens in lower case and reads as a name
// in code. It carries a capital, a digit, an underscore or a dot inside it. A
// single letter other than the article is a variable, such as q.
func lowerIdentifier(word string) bool {
	first, width := utf8.DecodeRuneInString(word)
	if !unicode.IsLower(first) {
		return false
	}
	if strings.TrimRight(word[width:], ".,;:!?)") == "" {
		return first != 'a'
	}
	return strings.IndexFunc(strings.TrimRight(word[width:], ".,;:!?)"), func(r rune) bool {
		return unicode.IsUpper(r) || unicode.IsDigit(r) || r == '_' || r == '.'
	}) >= 0
}
