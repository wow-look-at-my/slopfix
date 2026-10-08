package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/concurrency: a workflow with no concurrency block, so a push starts a second
// run beside the first. The repair writes the block these repositories carry.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDConcurrency,
		Category: RuleWorkflow,
		Detect:   detectConcurrency,
		Autofix:  autofixConcurrency,
		Cases:    []RuleCase{workflowCase("concurrency", "name: CI\n\non:\n  push:\n    branches: ['**']\n\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hi\n")},
	})
}

// detectConcurrency answers the workflow this case reports without a gate.
func detectConcurrency(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDConcurrency)
}

// autofixConcurrency writes the concurrency block for the workflow.
func autofixConcurrency(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDConcurrency)
}
