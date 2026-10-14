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
		Cases: []RuleCase{
			workflowCase("org-action-ref", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+
				"    steps:\n      - uses: wow-look-at-my/slopfix@v2\n"),
			// From wow-look-at-my/zxt: GitHub cannot resolve `<sha>#latest`, so every job failed at "Set up job".
			workflowCase("org-action-ref-sha-latest", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+
				"    steps:\n      - uses: wow-look-at-my/actions@a9b20e200c7edc8e7bc23fbafbe5ae87eb5f2410#latest # cache-upload#latest\n"+
				"        with:\n          key: zxt-bin-${{ github.sha }}\n          path: build/zxt\n"),
			workflowCase("org-action-ref-sha-pin", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+
				"    steps:\n      - uses: wow-look-at-my/actions@f294f0c3c3873f1452bdb965c761580d7bcf625d # cache-download#latest\n"),
		},
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
