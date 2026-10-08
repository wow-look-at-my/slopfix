package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/paragraph-length: a paragraph over the sentence cap. The repair divides
// it where the cap is reached, so each paragraph a reader meets is short.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDParagraphLength,
		Category: RuleSTE,
		Detect:   detectParagraphLength,
		Autofix:  autofixParagraphLength,
		Cases: []RuleCase{{Name: ste.IDParagraphLength, Path: "w.md",
			Text: "The gate reads. The gate writes. The gate waits. The gate starts. " +
				"The gate stops. The gate opens. The gate closes.\n"}},
	})
}

// detectParagraphLength answers every paragraph over the sentence cap.
func detectParagraphLength(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDParagraphLength)
}

// autofixParagraphLength divides a paragraph where the cap is reached.
func autofixParagraphLength(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDParagraphLength)
}
