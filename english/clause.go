// clause.go answers where a match's subject stands, so a rule keyed to a past
// tense verb reads the sentence rather than the words alone.
//
// A change stated as the sentence's own assertion says the source tree stands
// somewhere else now, and that is a tombstone. A subordinator ahead of the same
// verb, or a noun the clause hangs off, makes it a condition on a value the
// running code already handled, which is the code the reader has.
//
// The reading is the word classes english.xml declares.
package english

import "strings"

// The subject positions a <pattern> may declare.
const (
	SubjectMatched   = "matched"
	SubjectPreceding = "preceding"
)

// SubjectPositions names every position a pattern may declare, so a table
// spelling a position wrong stops the program at its earliest read.
func SubjectPositions() []string {
	return []string{SubjectMatched, SubjectPreceding}
}

// asserts reports whether the clause a match heads at `at` states the
// sentence's own assertion.
func asserts(position, s string, at int) bool {
	before := clauseWords(s[:at])
	if position == SubjectMatched {
		// A noun against the match's left side makes the clause a modifier of that noun.
		return len(before) == 0 || !is(before[len(before)-1], "noun")
	}
	i := len(before)
	for i > 0 {
		word := before[i-1]
		i--
		if is(word, "article") || is(word, "determiner") || is(word, "pronoun") {
			break // A determiner or a bare pronoun closes the noun phrase.
		}
		if !is(word, "noun") {
			return true // No noun phrase stands here, so no clause encloses it.
		}
	}
	if i == 0 {
		return true // The subject opens the sentence.
	}
	head := before[i-1]
	return !is(head, "relative") && !is(head, "conjunction") && !is(head, "noun")
}

// clauseWords answers the words standing between the opening of the sentence
// the match sits in and the match itself.
func clauseWords(before string) []string {
	if cut := lastStop(before); cut >= 0 {
		before = before[cut+1:]
	}
	return strings.Fields(before)
}

func lastStop(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != '.' && s[i] != '!' && s[i] != '?' {
			continue
		}
		// A period with a word against its right side opens a file name.
		if i+1 < len(s) && !isSpaceByte(s[i+1]) {
			continue
		}
		return i
	}
	return -1
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// edges are the characters a word carries that no class claims.
const edges = ".,;:!?()[]{}\"'`"

// is reports whether a word belongs to a class the table declares.
func is(word, class string) bool {
	return classes.Is(strings.Trim(strings.ToLower(word), edges), class)
}
