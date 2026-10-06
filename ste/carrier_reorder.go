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
	return reorderNoComma(source, whole)
}

// reorderNoComma moves an opening subordinate clause behind its main clause
// when no comma separates them: "So if X Y" becomes "Y. This happens if X."
// The verb's own object is the noun phrase against the verb; the main clause
// opens at the next noun phrase that a finite verb follows.
func reorderNoComma(source string, whole *syntax.Sentence) (string, bool) {
	sub, ok := firstSubordinate(whole)
	if !ok || sub.Verb == nil {
		return source, false
	}
	j := sub.Verb.Last + 1
	// The verb's object sits directly against it, and belongs to the subordinate.
	if ph, ok := whole.PhraseAt(j); ok && ph.Kind == syntax.NounPhrase && ph.First == j {
		j = ph.Last + 1
	}
	for ; j < len(whole.Words); j++ {
		if !opensSubject(whole, j) {
			continue
		}
		if ph, ok := whole.PhraseAt(j); !ok || ph.Kind != syntax.NounPhrase || ph.First != j {
			continue
		}
		start := whole.Words[j].Start
		// The subordinate clause cannot end on a word that opens what follows it:
		// "if X waits and" leaves the conjunction stranded.
		if w, ok := lastWordBefore(whole, start); ok && danglingTags.Contains(w.Tag) {
			continue
		}
		head := strings.TrimSpace(source[:start])
		main := strings.TrimSpace(source[start:])
		if !opensClause(syntax.Parse(checkMask(main), nil)) {
			continue
		}
		ms := syntax.Parse(checkMask(main), nil)
		verb, ok := mainVerb(ms, len(main))
		if !ok {
			continue
		}
		stop := "."
		if n := len(main); n > 0 && strings.ContainsAny(main[n-1:], ".!?") {
			stop, main = main[n-1:], main[:n-1]
		}
		return capitalizeOpening(main) + stop + " " + carrierFor(ms, verb) + " " + lowerFirst(stripLeadingAdverb(head)) + ".", true
	}
	return source, false
}

// firstSubordinate answers the opening subordinate clause of a sentence.
func firstSubordinate(whole *syntax.Sentence) (syntax.Clause, bool) {
	for _, c := range whole.Clauses {
		if c.Kind == syntax.Subordinate && c.Depth == 1 {
			return c, true
		}
	}
	return syntax.Clause{}, false
}

// stripLeadingAdverb drops a sentence-opening adverb such as "So" from the
// subordinate clause that moves behind the main one.
func stripLeadingAdverb(text string) string {
	fields := strings.Fields(text)
	if len(fields) > 1 && leadingAdverbs.Contains(strings.ToLower(fields[0])) {
		return strings.TrimSpace(text[len(fields[0]):])
	}
	return text
}

// leadingAdverbs introduce a statement before its subordinate clause.
var leadingAdverbs = set.Of("so", "then", "thus", "also", "therefore")

// opensVerb reports text whose first word is a verb, so a verb group that
// shares. The sentence's subject opens a sentence with the subject named
// again.
func opensVerb(text string) bool {
	s := syntax.Parse(checkMask(text), nil)
	if len(s.Words) == 0 {
		return false
	}
	t, w := s.Words[0].Tag, strings.ToLower(s.Words[0].Text)
	// The tagger reads an -s verb after "and" as a plural noun, the same way it reads "goal mode blocks".
	return strings.HasPrefix(t, "VB") || t == "MD" || t == "NNS" && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss")
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
