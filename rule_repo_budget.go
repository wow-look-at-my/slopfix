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
			ClaudeFile: ClaudeStub,
			AgentsFile: overBudget(),
		}}},
	})
}

// budgetNouns name the sections of the budget case and the things each
// paragraph talks about, so no sections share a line.
var budgetNouns = []string{"cache", "index", "queue", "ledger", "socket", "buffer", "schema", "parser",
	"router", "planner", "loader", "signer", "mapper", "packer", "reader", "writer"}

// overBudget is an AGENTS.md past CharBudget, in sections of short clean
// paragraphs, so the size is the only thing a rule reads in it.
func overBudget() string {
	out := "# Agents\n\nEach section names a step.\n"
	for _, topic := range budgetNouns {
		out += "\n## The " + topic + " step\n"
		for _, noun := range budgetNouns {
			out += "\nThe " + topic + " step reads the " + noun + ". It writes the " + noun + " to the disk. " +
				"The " + topic + " step then checks the " + noun + ". A bad " + noun + " stops the " + topic + " step. " +
				"The " + topic + " step logs each " + noun + ". The log shows the " + noun + ".\n"
		}
	}
	return out
}

// detectBudget answers every root file over the budget.
func detectBudget(c RuleCase) []ste.Finding { return treeFindings(c, IDBudget) }

// autofixBudget moves the file's largest sections into docs/.
func autofixBudget(c RuleCase) RuleCase { return treeAutofix(c, IDBudget) }
