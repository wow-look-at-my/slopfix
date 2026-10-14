package slopfix

import (
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// comments/number: a number in a comment is a count of what exists today. The
// repair says it in words, or names the item the number was counting.
func init() {
	RegisterRule(RuleSpec{
		ID:       commentfix.ID,
		Category: RuleComments,
		Detect:   detectCommentNumber,
		Autofix:  autofixCommentNumber,
		Cases: []RuleCase{
			{Name: commentfix.ID, Path: "main.go", Text: "package main\n\n// There are 3 modes today.\nfunc main() {}\n"},
			{
				Name: "a verb before a zero count is no label", Path: "gate.cmake",
				Text: "# Each run must exit 0, report the registry slots and the stripe mode, report 0 validation errors and write its PNG.\nset(A 1)\n",
				Want: "# Each run must exit 0, report the registry slots and the stripe mode, report no validation errors and write its PNG.\nset(A 1)\n",
			},
			{
				Name: "an exit status wrapped onto the next comment line", Path: "gate.h", Unchanged: true,
				Text: "// Under --validate the validation errors of the device phases fail the run (exit\n// 2), the convention of the shed test.\nint runRecreateTest();\n",
			},
		},
	})
}

// detectCommentNumber answers every number this rule reports in a comment.
func detectCommentNumber(c RuleCase) []ste.Finding {
	return caseFindings(c, commentfix.ID)
}

// autofixCommentNumber says the number in words, or names the item it counted.
func autofixCommentNumber(c RuleCase) RuleCase {
	return caseAutofix(c, commentfix.ID)
}
