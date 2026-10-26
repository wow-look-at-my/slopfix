// ste/tense: STE allows the simple tenses only.

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
		Cases: []RuleCase{
			{Name: ste.IDTense, Path: "w.md", Text: "The gate has started the task.\n", Want: "The gate started the task.\n"},
			// A progressive with you as its subject became "You looks for".
			{Name: "progressive-after-you", Path: "w.md", Text: "You are looking for the actions a careful developer will not take alone.\n", Want: "You look for the actions a careful developer will not take alone.\n"},
		},
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
