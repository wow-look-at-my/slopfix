package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/push-tags: a push trigger that runs on a tag as well as on a branch.
// Which starts a full matrix for a tag nobody builds. The repair adds the
// branch filter.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDPushTags,
		Category: RuleWorkflow,
		Detect:   detectPushTags,
		Autofix:  autofixPushTags,
		Cases:    []RuleCase{workflowCase("push-tags", "name: CI\n\non:\n  push:\n\njobs:\n  build:\n    runs-on: ubuntu-latest\n"+workflowGate+"    steps:\n      - run: echo hi\n")},
	})
}

// detectPushTags answers every push trigger with no branch filter.
func detectPushTags(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDPushTags)
}

// autofixPushTags adds the branch filter to the push trigger.
func autofixPushTags(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDPushTags)
}
