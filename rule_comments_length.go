package slopfix

import (
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// comments/length: a comment run weighed against the construct beneath it. The
// repair cuts from the end, because a comment leads with its point.
func init() {
	RegisterRule(RuleSpec{
		ID:       commentfix.IDLength,
		Category: RuleComments,
		Detect:   detectCommentLength,
		Autofix:  autofixCommentLength,
		Cases:    []RuleCase{{Name: commentfix.IDLength, Path: "main.go", Text: "package main\n\n" + overlongComment() + "func main() {}\n"}},
	})
}

// detectCommentLength answers every overlong comment run this rule reports.
func detectCommentLength(c RuleCase) []ste.Finding {
	return caseFindings(c, commentfix.IDLength)
}

// autofixCommentLength cuts the run from its end down to the budget.
func autofixCommentLength(c RuleCase) RuleCase {
	return caseAutofix(c, commentfix.IDLength)
}

// overlongComment is a Go comment far longer than the code beneath it, and
// under the volume cap. Only its weight against the code reads it.
func overlongComment() string {
	return "// The loop reads each value from the input and adds it to the total.\n" +
		"// It then checks the total against the limit that the caller set.\n" +
		"// A total over the limit stops the loop before it writes anything.\n" +
		"//\n" +
		"// The caller reads the total after the loop stops.\n" +
		"// The caller then writes the total to the log.\n"
}
