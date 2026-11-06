// yaml/id-token: a job whose permissions block does not grant id-token: write.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/id-token: a job whose permissions block does not grant id-token: write. The repair writes the grant.
func init() {
	job := func(perms string) string {
		return workflowHeader() + embeddedYml17 + perms + "    steps:\n      - run: make\n"
	}
	RegisterRule(RuleSpec{
		ID:       workflow.IDIDToken,
		Category: RuleWorkflow,
		Detect:   detectIDToken,
		Autofix:  autofixIDToken,
		Cases: []RuleCase{
			{
				Name: "yaml/id-token-job-block", Path: ".github/workflows/ci.yml",
				Text: job(embeddedYml18),
				Want: job(embeddedYml19),
			},
			{
				Name: "yaml/id-token-read", Path: ".github/workflows/ci.yml",
				Text: job(embeddedYml20),
				Want: job(embeddedYml21),
			},
			{
				Name: "yaml/id-token-flow", Path: ".github/workflows/ci.yml",
				Text: job("    permissions: {contents: read}\n"),
				Want: job("    permissions: {id-token: write, contents: read}\n"),
			},
			{
				Name: "yaml/id-token-empty-flow", Path: ".github/workflows/ci.yml",
				Text: job("    permissions: {}\n"),
				Want: job("    permissions: {id-token: write}\n"),
			},
			{
				Name: "yaml/id-token-workflow-block", Path: ".github/workflows/ci.yml",
				Text: workflowHeader() + embeddedYml2 +
					embeddedYml22,
				Want: workflowHeader() + embeddedYml3 +
					embeddedYml23,
			},
		},
	})
}

// detectIDToken answers the permissions blocks this case leaves without id-token: write.
func detectIDToken(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDIDToken)
}

// autofixIDToken writes id-token: write into each block.
func autofixIDToken(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDIDToken)
}

//go:embed testdata/embedded-2.yml
var embeddedYml2 string

//go:embed testdata/embedded-3.yml
var embeddedYml3 string

//go:embed testdata/embedded-11.yml
var embeddedYml17 string

//go:embed testdata/embedded-17.yml
var embeddedYml18 string

//go:embed testdata/embedded-18.yml
var embeddedYml19 string

//go:embed testdata/embedded-19.yml
var embeddedYml20 string

//go:embed testdata/embedded-20.yml
var embeddedYml21 string

//go:embed testdata/embedded-21.yml
var embeddedYml22 string

//go:embed testdata/embedded-22.yml
var embeddedYml23 string
