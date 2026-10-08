package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/modal: STE approves can, must and will, and bans the hedges that leave a
// reader unsure whether the instruction binds. The repair writes the approved
// modal.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDModal,
		Category: RuleSTE,
		Detect:   detectModal,
		Autofix:  autofixModal,
		Cases:    []RuleCase{{Name: ste.IDModal, Path: "x.md", Text: "The tool should write the result.\n"}},
	})
}

// detectModal answers each banned modal the case carries.
func detectModal(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDModal)
}

// autofixModal writes the approved modal for every banned one, and answers the
// case without the finding.
func autofixModal(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDModal)
}
