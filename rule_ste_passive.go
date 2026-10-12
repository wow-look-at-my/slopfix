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
			{Name: ste.IDPassive, Path: "w.md", Text: "The task is started by the gate.\n"},
			// dats docs/cli.md: an opening adverb keeps its place, and a verb the table does not list keeps its e.
			{Name: ste.IDPassive + " after an opening adverb", Path: "w.md", Text: "Otherwise the runs are separated by a dashed line.\n"},
			// A plural common noun names the actor as well as a name does.
			{Name: ste.IDPassive + " with a common-noun actor", Path: "w.md", Text: "The task is started by users.\n"},
			// Bold text goes back around the words it marked, wherever the rewrite moves them.
			{Name: ste.IDPassive + " with bold text", Path: "w.md", Text: "The task is **started** by the gate.\n"},
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
