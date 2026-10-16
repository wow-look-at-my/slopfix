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
			{Name: "label", Path: "w.md", Text: "The rule: a file must never be lost by a command the agent runs.\n", Want: "The rule: a command the agent runs must never lose a file.\n"},
			{Name: "modal and never", Path: "w.md", Text: "The file must never be edited by the agent.\n", Want: "The agent must never edit the file.\n"},
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
