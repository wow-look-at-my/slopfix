// carrier_phrases.go holds the phrase helpers of a carrier division.

package ste

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// oxford reports a list that puts a comma before its conjunction.
func oxford(rest string) bool {
	return strings.Contains(rest, ", and ") || strings.Contains(rest, ", or ")
}

// listHead ends the items before a list cut with the list's conjunction:
// "A, B, C," becomes "A, B and C". It joins at the last comma past the verb
// that no parenthesis or code span holds.
func listHead(head, conj string, serial bool, s *syntax.Sentence, verb syntax.Phrase) string {
	head = strings.TrimRight(head, " ,")
	from := s.Words[verb.Last].End
	if colon := strings.LastIndex(head, ":"); colon > from {
		from = colon
	}
	commas := topCommas(head, from)
	if len(commas) == 0 {
		return head
	}
	at := commas[len(commas)-1]
	// The last item already has its conjunction: "hooks, and `ask` rules".
	if leadingConjunction.MatchString(strings.ToLower(strings.TrimLeft(head[at+1:], " "))) {
		return head
	}
	join := " " + conj + " "
	if serial && len(commas) > 1 {
		join = ", " + conj + " "
	}
	return head[:at] + join + strings.TrimLeft(head[at+1:], " ")
}

// topCommas answers each comma of text from byte from on that no parenthesis,
// bracket or code span holds.
func topCommas(text string, from int) []int {
	var out []int
	depth, code := 0, false
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '`':
			code = !code
		case '(', '[':
			depth++
		case ')', ']':
			depth = max(depth-1, 0)
		case ',':
			if i >= from && depth == 0 && !code {
				out = append(out, i)
			}
		}
	}
	return out
}

// standsWithMainClause reports text that stands alone and, when it opens on a
// preposition, holds a finite verb outside a relative clause.
func standsWithMainClause(rest string) bool {
	if !StandsAlone(rest) {
		return false
	}
	if startsWithPreposition(rest) {
		return finiteOutsideRelative(syntax.Parse(checkMask(rest), nil), 0)
	}
	return true
}

// relativeTail reports the words from i to the end as the rest of a relative
// clause whose antecedent sits behind the main verb.
func relativeTail(s *syntax.Sentence, i int) bool {
	for _, w := range s.Words[min(i, len(s.Words)):] {
		if w.Tag == "CC" || w.Text == ":" || w.Text == ";" {
			return false
		}
	}
	return true
}

// startsWithPreposition reports text whose first word is a preposition.
func startsWithPreposition(text string) bool {
	s := syntax.Parse(checkMask(text), nil)
	return len(s.Words) > 0 && (s.Words[0].Tag == "IN" || s.Words[0].Tag == "TO")
}
