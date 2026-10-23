package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// counts/stated-count: a stated count of items goes stale when somebody adds the item that makes it wrong. The repair writes words that state no figure. The case carries no frame, which keeps it to this
// rule rather than the document count rule.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDStaleCount,
		Category: RuleCounts,
		Detect:   detectStaleCount,
		Autofix:  autofixStaleCount,
		Cases: []RuleCase{
			{Name: ste.IDStaleCount, Path: "x.md", Text: "The gate reads 3 rows.\n"},
			{
				Name: "a duration with a unit abbreviation is written out", Path: "supervisor.md",
				Text: "The supervisor sends SIGTERM when the budget runs out. A job still alive 15 s later exits 137.\n",
				Want: "The supervisor sends SIGTERM when the budget runs out. A job still alive several seconds later exits 137.\n",
			},
			{
				Name: "a measure before its noun leaves no unit behind", Path: "jobs.md",
				Text: "Idle 30 s jobs show no drift.\n",
				Want: "Idle jobs show no drift.\n",
			},
			{
				Name: "a count of past events keeps an amount", Path: "msg.txt",
				Text: "The monitor logged 2 samples before the outage.\n",
				Want: "The monitor logged a couple of samples before the outage.\n",
			},
			{
				Name: "a count that opens a subject keeps a determiner", Path: "x.md",
				Text: "The run passed, and 17 gates sat \"Skipped\" in a green suite.\n",
				Want: "The run passed, and the gates sat \"Skipped\" in a green suite.\n",
			},
		},
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
