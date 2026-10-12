package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/dictionary: a word the STE dictionary does not approve.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDDictionary,
		Category: RuleSTE,
		Detect:   detectDictionary,
		Autofix:  autofixDictionary,
		Cases:    []RuleCase{{Name: ste.IDDictionary, Path: "w.md", Text: "The tool gives additional output today.\n"}},
	})
}

// detectDictionary answers every word the STE dictionary does not approve.
func detectDictionary(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDDictionary)
}

// autofixDictionary writes the approved word where the table names one.
func autofixDictionary(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDDictionary)
}
