package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/passive: STE prefers the active voice, because the reader has to find who
// acts. The repair names the actor as the subject.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDPassive,
		Category: RuleSTE,
		Detect:   detectPassive,
		Autofix:  autofixPassive,
		Cases: []RuleCase{
			{Name: ste.IDPassive, Path: "w.md", Text: "The task is started by the gate.\n", Want: "The gate starts the task.\n"},
			{Name: ste.IDPassive + "/modal", Path: "modal.md", Text: "A file must be read by the gate.\n", Want: "The gate must read a file.\n"},
			{Name: ste.IDPassive + "/means", Path: "means.md", Text: "An answer is asserted by pairing it with `uname -s`.\n", Want: "Pairing it with `uname -s` asserts an answer.\n"},
			{Name: ste.IDPassive + "/manner", Path: "manner.md", Text: "The sites are deduplicated by full URL.\n", Want: "By full URL, the sites are deduplicated.\n"},
			{Name: ste.IDPassive + "/manner-after-subordinator", Path: "clause.md", Text: "The build stops because the sites are deduplicated by full URL.\n", Want: "The build stops because by full URL the sites are deduplicated.\n"},
			{Name: ste.IDPassive + "/no-by-phrase", Path: "plain.md", Text: "The task is started.\n", Unchanged: true},
			{Name: "an adjective after be is no passive", Path: "w.md", Text: "The task is started by the gate. Floaters cannot be solid by definition.\n",
				Want: "The gate starts the task. Floaters cannot be solid by definition.\n"},
		},
	})
}

// detectPassive answers every passive clause this case reports.
func detectPassive(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDPassive)
}

// autofixPassive writes the active voice for a clause that names its actor.
func autofixPassive(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDPassive)
}
