package slopfix

import (
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/ste"
)

// counts/section-number: a section number goes stale when a section is inserted
// above it. The repair writes a link named by a slug of the heading's title.
func init() {
	RegisterRule(RuleSpec{
		ID:       counts.IDSection,
		Category: RuleCounts,
		Detect:   detectSectionNumber,
		Autofix:  autofixSectionNumber,
		Cases:    []RuleCase{{Name: counts.IDSection, Path: "x.md", Text: "# 9. Owning the renderer\n\nSee §9 for the detail.\n"}},
	})
}

// detectSectionNumber answers every stale section citation this rule reports.
func detectSectionNumber(c RuleCase) []ste.Finding {
	return caseFindings(c, counts.IDSection)
}

// autofixSectionNumber writes a link named by a slug of the heading's title.
func autofixSectionNumber(c RuleCase) RuleCase {
	return caseAutofix(c, counts.IDSection)
}
