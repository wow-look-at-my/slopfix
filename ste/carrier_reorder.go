// carrier_reorder.go moves an opening dependent clause behind the main clause
// it belongs to, as a division of its own.
package ste

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// reorderDependent moves an opening subordinate clause behind its main clause,
// into a sentence of its own: "If X, Y." becomes "Y. This happens if X." A main
// clause that points back into the subordinate clause, as "those files" does,
// keeps the order: "If X, Y." becomes "Suppose X. Then Y."
func reorderDependent(source string, whole *syntax.Sentence) (string, bool) {
	if !opensDependent(whole) {
		return source, false
	}
	// The main clause opens after the earliest comma outside a parenthesis that leaves a sentence: "To do X, or to do Y, see Z".
	depth := 0
	for comma, w := range whole.Words {
		switch w.Text {
		case "(":
			depth++
		case ")":
			depth = max(depth-1, 0)
		}
		if w.Text != "," || depth > 0 || comma < 1 || comma+1 >= len(whole.Words) {
			continue
		}
		sub := strings.TrimSpace(source[:w.Start])
		main := strings.TrimSpace(source[w.End:])
		imperative := opensImperative(checkMask(main))
		if !StandsAlone(main) && !imperative {
			continue
		}
		stop := "."
		if n := len(main); n > 0 && strings.ContainsAny(main[n-1:], ".!?") {
			stop, main = main[n-1:], main[:n-1]
		}
		ms := syntax.Parse(checkMask(main), nil)
		carrier := "Do this"
		if verb, ok := mainVerb(ms, len(main)); ok && !(imperative && !verb.Imperative) {
			carrier = carrierFor(ms, verb)
		} else if !imperative {
			continue
		}
		link := strings.ToLower(firstToken.FindString(sub))
		if (link == "if" || link == "when" || link == "whenever") && pointsBack(ms) {
			return "Suppose " + strings.TrimSpace(sub[len(link):]) + ". Then " + lowerFirst(main) + stop, true
		}
		return capitalizeOpening(main) + stop + " " + carrier + " " + lowerFirst(sub) + ".", true
	}
	return source, false
}

// backReferences are the words that point back to a noun said before them.
var backReferences = set.Of("those", "these", "this", "that", "it", "its", "they", "them", "their", "such")

// pointsBack reports a clause whose first words point back to a noun said before them.
func pointsBack(s *syntax.Sentence) bool {
	for _, w := range s.Words[:min(3, len(s.Words))] {
		if backReferences.Contains(w.Lower()) {
			return true
		}
	}
	return false
}

// enoughBefore reports "enough" among the couple of words before word i, which makes a that-clause at i a result.
func enoughBefore(s *syntax.Sentence, i int) bool {
	for k := max(i-2, 0); k < i; k++ {
		if s.Words[k].Lower() == "enough" {
			return true
		}
	}
	return false
}

// lowerFirst writes the first letter in lower case, unless the word is a name in capitals.
func lowerFirst(s string) string {
	if len(s) > 1 && unicode.IsUpper(rune(s[1])) {
		return s
	}
	return lowerOpening(s)
}
