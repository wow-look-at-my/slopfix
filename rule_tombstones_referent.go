package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// tombstones/name-nothing-in-the-repository-defines: a comment that names a
// symbol no file of the repository holds. The repair strips the comment line,
// or cuts the sentence that holds the name.
func init() {
	RegisterRule(RuleSpec{
		ID:       tombstones.IDDeadReferent,
		Category: RuleTombstones,
		Detect:   detectDeadReferent,
		Autofix:  autofixDeadReferent,
		Cases: []RuleCase{{
			Name:  tombstones.IDDeadReferent,
			Path:  "main.go",
			Text:  "package main\n\n// parseLegacyFlag reads the input.\nfunc main() {}\n",
			Files: map[string]string{"util.go": "package main\n\nfunc readInput() {}\n"},
		}},
	})
}

// detectDeadReferent answers every name a comment carries that no file holds.
func detectDeadReferent(c RuleCase) []ste.Finding {
	return caseFindings(c, tombstones.IDDeadReferent)
}

// autofixDeadReferent strips the comment line or cuts the sentence that holds
// the name.
func autofixDeadReferent(c RuleCase) RuleCase {
	return caseAutofix(c, tombstones.IDDeadReferent)
}
