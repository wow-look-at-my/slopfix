package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/instruction-length: an instruction over the word cap. The repair divides
// it into sentences of the cap or less, so each step a reader follows is short.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDInstructionLength,
		Category: RuleSTE,
		Detect:   detectInstructionLength,
		Autofix:  autofixInstructionLength,
		Cases: []RuleCase{{Name: ste.IDInstructionLength, Path: "w.md",
			Text: "The gate reads the task and the caller waits for it before the next step " +
				"starts and the count goes to the table.\n"}},
	})
}

// detectInstructionLength answers every instruction over the word cap.
func detectInstructionLength(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDInstructionLength)
}

// autofixInstructionLength divides an instruction into sentences of the cap.
func autofixInstructionLength(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDInstructionLength)
}
