package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/org-action-ref: an org action referenced at a version tag or a branch of
// one person's making. The repair points every org action at master.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDOrgActionRef,
		Category: RuleWorkflow,
		Detect:   detectOrgActionRef,
		Autofix:  autofixOrgActionRef,
		Cases: []RuleCase{workflowCase("org-action-ref", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+
			"    steps:\n      - uses: wow-look-at-my/slopfix@v2\n")},
	})
}

// detectOrgActionRef answers every org action pinned to a tag or branch.
func detectOrgActionRef(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDOrgActionRef)
}

// autofixOrgActionRef points each org action reference at master.
func autofixOrgActionRef(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDOrgActionRef)
}
