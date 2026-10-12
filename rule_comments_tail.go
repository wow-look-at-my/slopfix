package slopfix

import (
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// comments/tail: a comment that stops on a word which opens what a cut took
// away, as in a comment ending on "because". The repair closes the sentence
// where its last clause that stands alone ends.
func init() {
	RegisterRule(RuleSpec{
		ID:       commentfix.IDTail,
		Category: RuleComments,
		Detect:   detectCommentTail,
		Autofix:  autofixCommentTail,
		Cases:    []RuleCase{{Name: commentfix.IDTail, Path: "main.go", Text: "package main\n\n// The loop reads each value because\nfunc main() {}\n"}},
	})
}

// detectCommentTail answers every tail this rule reports.
func detectCommentTail(c RuleCase) []ste.Finding {
	return caseFindings(c, commentfix.IDTail)
}

// autofixCommentTail closes the sentence where its last clause that stands
// alone ends.
func autofixCommentTail(c RuleCase) RuleCase {
	return caseAutofix(c, commentfix.IDTail)
}
