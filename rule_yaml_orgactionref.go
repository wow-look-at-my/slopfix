// yaml/org-action-ref: an org action referenced at a version tag, a SHA or a branch, its default branch included.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/org-action-ref: an org action referenced at a version tag, a SHA or a
// branch, its default branch included. The `lkgb` tag passes. The repair points
// the action at `lkgb`, or at the default branch of a repository that
// publishes no `lkgb`.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDOrgActionRef,
		Category: RuleWorkflow,
		Detect:   detectOrgActionRef,
		Autofix:  autofixOrgActionRef,
		Cases: []RuleCase{
			workflowCase("org-action-ref", workflowHeader()+embeddedYml26+
				"    steps:\n      - uses: wow-look-at-my/slopfix@v2\n"),
			// From wow-look-at-my/zxt: GitHub cannot resolve `<sha>#latest`, so every job failed at "Set up job".
			workflowCase("org-action-ref-sha-latest", workflowHeader()+embeddedYml27+
				"    steps:\n      - uses: wow-look-at-my/actions@a9b20e200c7edc8e7bc23fbafbe5ae87eb5f2410#latest # cache-upload#latest\n"+
				embeddedYml28),
			workflowCase("org-action-ref-sha-pin", workflowHeader()+embeddedYml29+
				"    steps:\n      - uses: wow-look-at-my/actions@f294f0c3c3873f1452bdb965c761580d7bcf625d # cache-download#latest\n"),
			workflowCase("org-action-ref-branch", workflowHeader()+embeddedYml30+
				"    steps:\n      - uses: wow-look-at-my/slopfix@one-person-feature\n"),
			workflowCase("org-action-ref-bare-sha", workflowHeader()+embeddedYml31+
				"    steps:\n      - uses: wow-look-at-my/buildhost@f294f0c3c3873f1452bdb965c761580d7bcf625d\n"),
			lkgbCase(),
			goneBranchCase(),
			workflowCase("org-action-ref-default-branch", workflowHeader()+embeddedYml32+
				"    steps:\n      - uses: wow-look-at-my/buildhost/.github/actions/buildhost-download@master\n"),
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

// goneBranchCase pins go-toolchain at v1, and the repair writes lkgb.
func goneBranchCase() RuleCase {
	steps := func(ref string) string {
		return workflowHeader() + goneBranchCaseYml +
			"    steps:\n      - uses: wow-look-at-my/go-toolchain@" + ref + "\n        with:\n          working-directory: go\n"
	}
	c := workflowCase("org-action-ref-gone-branch", steps("v1"))
	c.Want = steps("lkgb")
	return c
}

// detectOrgActionRef answers every org action pinned to a tag other than `lkgb`, a SHA or a branch.
func detectOrgActionRef(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDOrgActionRef)
}

// autofixOrgActionRef points each reported org action at `lkgb`.
func autofixOrgActionRef(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDOrgActionRef)
}

//go:embed testdata/embedded-11.yml
var embeddedYml26 string

//go:embed testdata/embedded-14.yml
var embeddedYml27 string

//go:embed testdata/embedded-24.yml
var embeddedYml28 string

//go:embed testdata/embedded-25.yml
var embeddedYml29 string

//go:embed testdata/embedded-26.yml
var embeddedYml30 string

//go:embed testdata/embedded-27.yml
var embeddedYml31 string

//go:embed testdata/embedded-28.yml
var embeddedYml32 string

//go:embed testdata/lkgb-case.yml
var lkgbCaseYml string

//go:embed testdata/lkgb-case-2.yml
var lkgbCaseYml2 string

//go:embed testdata/gone-branch-case.yml
var goneBranchCaseYml string
