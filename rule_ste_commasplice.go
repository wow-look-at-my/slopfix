package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// english/comma-splice: a comma joins clauses that each stand alone. The
// repair writes a period and capitalizes the next word.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDCommaSplice,
		Category: RuleEnglish,
		Detect:   detectCommaSplice,
		Autofix:  autofixCommaSplice,
		Cases:    []RuleCase{{Name: ste.IDCommaSplice, Path: "x.md", Text: "The gate reads the file, the tool writes the result.\n"}},
	})
}

// detectCommaSplice answers each comma joining clauses that the case carries.
func detectCommaSplice(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDCommaSplice)
}

// autofixCommaSplice writes a period and capitalizes the next word for every
// splice, and answers the case without the finding.
func autofixCommaSplice(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDCommaSplice)
}
