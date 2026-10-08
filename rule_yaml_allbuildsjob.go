package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/all-builds-job. A job named all-builds shadows the aggregate status the
// required-builds app posts, and the guard that owns the name rejects the run.
// The repair renames the job.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDAllBuildsJob,
		Category: RuleWorkflow,
		Detect:   detectAllBuildsJob,
		Autofix:  autofixAllBuildsJob,
		Cases:    []RuleCase{workflowCase("all-builds-job", workflowHeader()+"jobs:\n  all-builds:\n    runs-on: ubuntu-latest\n"+workflowGate+"    steps:\n      - run: echo hi\n")},
	})
}

// detectAllBuildsJob answers every job named all-builds.
func detectAllBuildsJob(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDAllBuildsJob)
}

// autofixAllBuildsJob renames the all-builds job.
func autofixAllBuildsJob(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDAllBuildsJob)
}
