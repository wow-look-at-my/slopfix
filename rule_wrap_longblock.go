package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// wrap/long-block: a paragraph or a list item over the block cap. The repair
// divides it into paragraphs at a sentence end, which is why the fixture is one
// long list item.
func init() {
	RegisterRule(RuleSpec{
		ID:       IDLongBlock,
		Category: RuleWrap,
		Detect:   detectLongBlock,
		Autofix:  autofixLongBlock,
		Cases:    []RuleCase{{Name: IDLongBlock, Path: "x.md", Text: "- " + repeatedPhrase("The gate reads the file. ", 120) + "\n"}},
	})
}

// detectLongBlock answers each block over the cap that the case carries.
func detectLongBlock(c RuleCase) []ste.Finding {
	return caseFindings(c, IDLongBlock)
}

// autofixLongBlock divides the over-long block at a sentence end, and answers
// the case without the finding.
func autofixLongBlock(c RuleCase) RuleCase {
	return caseAutofix(c, IDLongBlock)
}
