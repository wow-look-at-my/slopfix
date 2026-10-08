package slopfix

import (
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// comments/number: a number in a comment is a count of what exists today. The
// repair says it in words, or names the item the number was counting.
//
// The second case is a doc comment that states an exit code. An exit status is
// a value the program answers with, not a count of what exists here. No rule
// reports it and the repair returns the comment as the author wrote it.
func init() {
	RegisterRule(RuleSpec{
		ID:       commentfix.ID,
		Category: RuleComments,
		Detect:   detectCommentNumber,
		Autofix:  autofixCommentNumber,
		Cases: []RuleCase{
			{Name: commentfix.ID, Path: "main.go", Text: "package main\n\n// There are 3 modes today.\nfunc main() {}\n"},
			{Name: commentfix.ID + "/doc", Path: "stop.rs", Text: commentNumberDocCase(), Unchanged: true},
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

// commentNumberDocCase is a Rust doc comment that states an exit code.
func commentNumberDocCase() string {
	return "/// Dispatch the observe-only session-end `Stop`: runs in stop-gate mode so\n" +
		"/// exit code 2 parses as a block, but the decision is discarded (no turn\n" +
		"/// left to continue).\n" +
		"pub(crate) async fn dispatch_session_end_stop(&self, reason: &str) {\n" +
		"    if self.startup_hints.is_subagent {\n" +
		"        return;\n" +
		"    }\n" +
		"}\n"
}
