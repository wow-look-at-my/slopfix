package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// repo/budget: a root file over the character budget every request pays for.
// The repair moves its largest sections into docs/ beside it.
func init() {
	RegisterRule(RuleSpec{
		ID:       IDBudget,
		Category: RuleRepo,
		Detect:   detectBudget,
		Autofix:  autofixBudget,
		Cases: []RuleCase{{Name: IDBudget, Files: map[string]string{
			ClaudeFile: "## Topic\n\n" + repeatedPhrase("word ", CharBudget/5+10) + "\n",
		}}},
	})
}

// detectBudget answers every root file over the budget.
func detectBudget(c RuleCase) []ste.Finding { return treeFindings(c, IDBudget) }

// autofixBudget moves the file's largest sections into docs/.
func autofixBudget(c RuleCase) RuleCase { return treeAutofix(c, IDBudget) }
