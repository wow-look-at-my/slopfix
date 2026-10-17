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
		Cases: []RuleCase{
			{Name: ste.IDModal, Path: "x.md", Text: "The tool should write the result.\n"},
			// "would" takes the present tense of its verb group, never "will", which turns a condition into a promise.
			{Name: ste.IDModal + " with would", Path: "x.md", Text: "Without the cache the build would fail.\n"},
			{Name: ste.IDModal + " with would be", Path: "x.md", Text: "A cached tree would be faster.\n"},
			{Name: ste.IDModal + " with would have", Path: "x.md", Text: "The old runner would have dropped the job.\n"},
			{Name: ste.IDModal + " with would not", Path: "x.md", Text: "The gate would not accept the token.\n"},
			{Name: ste.IDModal + " in a question", Path: "x.md", Text: "In that case, would the gate accept the token? The gate reads the header.\n"},
		},
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
