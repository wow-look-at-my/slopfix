package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/contraction: a contraction hides the word it stands for, and a reader
// whose English is a second language reads the full form faster. The repair
// expands it.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDContraction,
		Category: RuleSTE,
		Detect:   detectContraction,
		Autofix:  autofixContraction,
		Cases:    []RuleCase{{Name: ste.IDContraction, Path: "x.md", Text: "The tool doesn't write the result.\n"}},
	})
}

// detectContraction answers each contraction the case carries.
func detectContraction(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDContraction)
}

// autofixContraction expands the contraction to its full form, and answers the
// case without the finding.
func autofixContraction(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDContraction)
}
