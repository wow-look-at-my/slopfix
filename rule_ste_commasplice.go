package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/comma-splice: a comma joining clauses is the semicolon STE bans,
// spelled differently. The repair writes a period and capitalizes the next
// word.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDCommaSplice,
		Category: RuleSTE,
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
