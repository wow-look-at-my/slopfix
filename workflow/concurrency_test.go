package workflow_test

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

const orgBlock = "concurrency:\n" +
	"  group: gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\n" +
	"  cancel-in-progress: ${{ github.ref != 'refs/heads/master' }}\n"

func concurrencyFindings(content string) []ste.Finding {
	var out []ste.Finding
	for _, f := range workflow.Check(content) {
		if f.ID == workflow.IDConcurrency {
			out = append(out, f)
		}
	}
	return out
}

func TestAWorkflowWithoutTheOrgBlockIsReported(t *testing.T) {
	cases := map[string]int{
		"on: push\njobs: {}\n":                             1,
		testAWorkflowWithoutTheOrgBlockIsReportedYml:       2,
		testAWorkflowWithoutTheOrgBlockIsReportedYml2:      2,
		testAWorkflowWithoutTheOrgBlockIsReportedYml3:      2,
		"on: push\n" + orgBlock + "  extra: x\njobs: {}\n": 2,
	}
	for content, line := range cases {
		found := concurrencyFindings(content)
		require.Len(t, found, 1, content)
		assert.Equal(t, line, found[0].Line, content)
		assert.Contains(t, found[0].Fix, "cancel-in-progress", content)
	}
}

func TestTheOrgBlockPasses(t *testing.T) {
	for _, content := range []string{
		"on: push\n" + orgBlock + "jobs: {}\n",
		testTheOrgBlockPassesYml,
		// A reusable workflow runs in the caller's context, so the same group deadlocks.
		"on: workflow_call\njobs: {}\n",
		testTheOrgBlockPassesYml2,
		"runs:\n  using: composite\n",
	} {
		assert.Empty(t, concurrencyFindings(content), content)
	}
}

func TestTheRepairWritesTheOrgBlock(t *testing.T) {
	cases := map[string]string{
		"on: workflow_dispatch\njobs: {}\n":   "on: workflow_dispatch\n" + orgBlock + "jobs: {}\n",
		"on: workflow_dispatch\n\njobs: {}\n": "on: workflow_dispatch\n\n" + orgBlock + "\njobs: {}\n",
		testTheRepairWritesTheOrgBlockYml:     "on:\n  pull_request:\n" + orgBlock + "# the jobs\njobs: {}\n",
		testTheRepairWritesTheOrgBlockYml2:    "on: workflow_dispatch\n" + orgBlock + "jobs: {}\n",
		testTheRepairWritesTheOrgBlockYml3:    "on: workflow_dispatch\n" + orgBlock + "jobs: {}\n",
	}
	for content, want := range cases {
		out := workflow.Fix(content, func(string) bool { return true }).Text
		assert.Equal(t, want, out, content)
		assert.Empty(t, concurrencyFindings(out), content)
	}
}

// The block goes after the blank line that ends the triggers. Every line of the
// comment after it stays.
func TestTheRepairKeepsTheCommentAfterTheTriggers(t *testing.T) {
	head := "on:\n  push:\n    branches: [\"**\"]\n  pull_request:\n\n"
	tail := "# go-toolchain@v1 requirements. contents: write is the dependency-graph\n" +
		"# submission (403 without it).\npermissions:\n  contents: write\njobs: {}\n"
	only := func(id string) bool { return id == workflow.IDConcurrency }
	assert.Equal(t, head+orgBlock+"\n"+tail, workflow.Fix(head+tail, only).Text)
}

func TestTheRepairKeepsTheFileIndent(t *testing.T) {
	content := testTheRepairKeepsTheFileIndentYml
	out := workflow.Fix(content, func(string) bool { return true }).Text
	assert.Contains(t, out, "concurrency:\n    group: gha_")
	assert.Contains(t, out, "\n    cancel-in-progress: ${{")
	assert.Empty(t, concurrencyFindings(out))
}

//go:embed testdata/test-a-workflow-without-the-org-block-is-reported.yml
var testAWorkflowWithoutTheOrgBlockIsReportedYml string

//go:embed testdata/test-a-workflow-without-the-org-block-is-reported-2.yml
var testAWorkflowWithoutTheOrgBlockIsReportedYml2 string

//go:embed testdata/test-a-workflow-without-the-org-block-is-reported-3.yml
var testAWorkflowWithoutTheOrgBlockIsReportedYml3 string

//go:embed testdata/test-the-org-block-passes.yml
var testTheOrgBlockPassesYml string

//go:embed testdata/test-the-org-block-passes-2.yml
var testTheOrgBlockPassesYml2 string

//go:embed testdata/test-the-repair-writes-the-org-block.yml
var testTheRepairWritesTheOrgBlockYml string

//go:embed testdata/test-the-repair-writes-the-org-block-2.yml
var testTheRepairWritesTheOrgBlockYml2 string

//go:embed testdata/test-the-repair-writes-the-org-block-3.yml
var testTheRepairWritesTheOrgBlockYml3 string

//go:embed testdata/test-the-repair-keeps-the-file-indent.yml
var testTheRepairKeepsTheFileIndentYml string
