// budget.go fits a prose run to a budget. It is the operation the sentence
// cap, the comment volume cap, the comment length rule and the count rule
// reach for: shorten the run. By tightening its words and restating its
// clauses, the operation sentence division already performs. It never deletes
// a sentence, so a run that cannot fit is left for a caller to cut rather
// than quietly losing what it said.
package ste

import "unicode"

// FitToBudget rewrites text to hold no more than budget non-whitespace
// characters, tightening the wording with tighten and dividing each sentence
// still over the cap. tighten may be nil, and then only the division runs. It
// answers the fitted text and whether it fits.
func FitToBudget(text string, budget int, tighten func(string) string) (string, bool) {
	fitted := text
	for range 8 {
		if BudgetChars(fitted) <= budget {
			return fitted, true
		}
		next := fitted
		if tighten != nil {
			next = tighten(next)
		}
		next = fixSentenceCap(next, capSpec{reorder: true, cap: SentenceWordCap})
		// A pass that removes no character has nothing left to give, so the
		// loop stops rather than spinning on the same text.
		if BudgetChars(next) >= BudgetChars(fitted) {
			return fitted, BudgetChars(fitted) <= budget
		}
		fitted = next
	}
	return fitted, BudgetChars(fitted) <= budget
}

// BudgetChars counts the non-whitespace characters of text, the measure the
// comment length rule weighs a block by.
func BudgetChars(text string) int {
	n := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}
