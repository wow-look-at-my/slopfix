package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/comment-block: more than one comment line in a run of them. The repair
// folds the run into one line, because a guard reads the second line as a
// block.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDCommentBlock,
		Category: RuleWorkflow,
		Detect:   detectCommentBlock,
		Autofix:  autofixCommentBlock,
		Cases:    []RuleCase{workflowCase("comment-block", workflowHeader()+"# first line\n# second line\n# third line\n\n"+workflowTail())},
	})
}

// detectCommentBlock answers every comment run over one line.
func detectCommentBlock(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDCommentBlock)
}

// autofixCommentBlock folds the comment run into one line.
func autofixCommentBlock(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDCommentBlock)
}
