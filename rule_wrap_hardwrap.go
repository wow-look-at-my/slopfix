package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// wrap/hard-wrap: a paragraph split over several source lines freezes one
// window's width into the file. Every later edit re-flows lines nobody
// touched. The repair joins the block back to a single line.
func init() {
	RegisterRule(RuleSpec{
		ID:       IDHardWrap,
		Category: RuleWrap,
		Detect:   detectHardWrap,
		Autofix:  autofixHardWrap,
		Cases:    []RuleCase{{Name: IDHardWrap, Path: "x.md", Text: "The gate reads the file\nand writes the result.\n"}},
	})
}

// detectHardWrap answers each paragraph the case wraps over several lines.
func detectHardWrap(c RuleCase) []ste.Finding {
	return caseFindings(c, IDHardWrap)
}

// autofixHardWrap joins the wrapped paragraph onto one line, and answers the
// case without the finding.
func autofixHardWrap(c RuleCase) RuleCase {
	return caseAutofix(c, IDHardWrap)
}
