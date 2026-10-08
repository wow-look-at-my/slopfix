package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/count: a stated count of items goes stale when somebody adds the item that makes it wrong. The repair writes words that state no figure. The case carries no frame, which keeps it to this
// rule rather than the document count rule.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDStaleCount,
		Category: RuleSTE,
		Detect:   detectStaleCount,
		Autofix:  autofixStaleCount,
		Cases:    []RuleCase{{Name: ste.IDStaleCount, Path: "x.md", Text: "The gate reads 3 rows.\n"}},
	})
}

// detectStaleCount answers each stated count the case carries.
func detectStaleCount(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDStaleCount)
}

// autofixStaleCount writes words that state no figure, and answers the case
// without the finding.
func autofixStaleCount(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDStaleCount)
}
