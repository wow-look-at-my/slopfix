package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/tense: STE allows the simple tenses only. The repair writes the simple
// past for a perfect clause, and the simple present for a progressive one.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDTense,
		Category: RuleSTE,
		Detect:   detectTense,
		Autofix:  autofixTense,
		Cases:    []RuleCase{{Name: ste.IDTense, Path: "w.md", Text: "The gate has started the task.\n"}},
	})
}

// detectTense answers every perfect or progressive clause this case reports.
func detectTense(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDTense)
}

// autofixTense writes the simple tense for a clause that carries a compound one.
func autofixTense(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDTense)
}
