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
				// From a splat2 AGENTS.md: the label repair wrote "A run still a later alive s later exits 124".
				Name: "a duration with a unit abbreviation is no count", Path: "watchdog.md", Unchanged: true,
				Text: "The watchdog sends SIGTERM when the budget runs out. A run still alive 15 s later exits 124.\n",
			},
			{
				// From splat2's docs/handoff.md: "Quiet 30 s runs" lost its number and kept the unit.
				Name: "a measure before its noun is no count", Path: "handoff.md", Unchanged: true,
				Text: "Quiet 30 s runs show no stall.\n",
			},
			{
				// From a splat2 commit message: "recorded 2 intervals" became "recorded intervals".
				Name: "a count of past events keeps an amount", Path: "msg.txt",
				Text: "The probe recorded 2 intervals before the stall.\n",
				Want: "The probe recorded a couple of intervals before the stall.\n",
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
