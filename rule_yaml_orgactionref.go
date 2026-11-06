// rule_yaml_orgactionref.go holds the rule yaml orgactionref code.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/org-action-ref: an org action referenced at a version tag, a SHA or a
// branch of one person's making. The `lkgb` tag passes. The repair points the
// action at its default branch.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDOrgActionRef,
		Category: RuleWorkflow,
		Detect:   detectOrgActionRef,
		Autofix:  autofixOrgActionRef,
		Cases: []RuleCase{
			workflowCase("org-action-ref", workflowHeader()+embeddedYml13+
				"    steps:\n      - uses: wow-look-at-my/slopfix@v2\n"),
			// From wow-look-at-my/zxt: GitHub cannot resolve `<sha>#latest`, so every job failed at "Set up job".
			workflowCase("org-action-ref-sha-latest", workflowHeader()+embeddedYml14+
				"    steps:\n      - uses: wow-look-at-my/actions@a9b20e200c7edc8e7bc23fbafbe5ae87eb5f2410#latest # cache-upload#latest\n"+
				embeddedYml15),
			workflowCase("org-action-ref-sha-pin", workflowHeader()+embeddedYml16+
				"    steps:\n      - uses: wow-look-at-my/actions@f294f0c3c3873f1452bdb965c761580d7bcf625d # cache-download#latest\n"),
			workflowCase("org-action-ref-branch", workflowHeader()+embeddedYml17+
				"    steps:\n      - uses: wow-look-at-my/slopfix@one-person-feature\n"),
			workflowCase("org-action-ref-bare-sha", workflowHeader()+embeddedYml18+
				"    steps:\n      - uses: wow-look-at-my/buildhost@f294f0c3c3873f1452bdb965c761580d7bcf625d\n"),
			lkgbCase(),
		},
	})
}

// lkgbCase is a workflow whose org actions sit at the `lkgb` tag, which the
// rule leaves as written.
func lkgbCase() RuleCase {
	c := workflowCase("org-action-ref-lkgb", workflowHeader()+lkgbCaseYml+
		lkgbCaseYml2)
	c.Unchanged = true
	return c
}

// detectOrgActionRef answers every org action pinned to a tag other than `lkgb`, a SHA or a branch.
func detectOrgActionRef(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDOrgActionRef)
}

// autofixOrgActionRef points each reported org action at its default branch.
func autofixOrgActionRef(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDOrgActionRef)
}

//go:embed testdata/embedded-13.yml
var embeddedYml13 string

//go:embed testdata/embedded-14.yml
var embeddedYml14 string

//go:embed testdata/embedded-15.yml
var embeddedYml15 string

//go:embed testdata/embedded-16.yml
var embeddedYml16 string

//go:embed testdata/embedded-17.yml
var embeddedYml17 string

//go:embed testdata/embedded-18.yml
var embeddedYml18 string

//go:embed testdata/lkgb-case.yml
var lkgbCaseYml string

//go:embed testdata/lkgb-case-2.yml
var lkgbCaseYml2 string
