package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/id-token: a job whose permissions block does not grant id-token: write. The repair writes the grant.
func init() {
	job := func(perms string) string {
		return workflowHeader() + "jobs:\n  build:\n    runs-on: ubuntu-latest\n" + perms + "    steps:\n      - run: make\n"
	}
	RegisterRule(RuleSpec{
		ID:       workflow.IDIDToken,
		Category: RuleWorkflow,
		Detect:   detectIDToken,
		Autofix:  autofixIDToken,
		Cases: []RuleCase{
			{
				Name: "yaml/id-token-job-block", Path: ".github/workflows/ci.yml",
				Text: job("    permissions:\n      contents: read\n      actions: write\n"),
				Want: job("    permissions:\n      id-token: write\n      contents: read\n      actions: write\n"),
			},
			{
				Name: "yaml/id-token-read", Path: ".github/workflows/ci.yml",
				Text: job("    permissions:\n      id-token: read # mint nothing\n      contents: read\n"),
				Want: job("    permissions:\n      id-token: write # mint nothing\n      contents: read\n"),
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
				Text: workflowHeader() + "permissions:\n  contents: read\n\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n" +
					"  b:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n",
				Want: workflowHeader() + "permissions:\n  id-token: write\n  contents: read\n\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n" +
					"  b:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make\n",
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
