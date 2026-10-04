package workflow_test

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
		"on: push\njobs: {}\n": 1,
		"name: ci\non:\n  push:\n    branches: ['**']\njobs: {}\n":                    2,
		"on: push\nconcurrency: ci\njobs: {}\n":                                       2,
		"on: push\nconcurrency:\n  group: ci\n  cancel-in-progress: true\njobs: {}\n": 2,
		"on: push\n" + orgBlock + "  extra: x\njobs: {}\n":                            2,
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
		"on: push\nconcurrency:\n  group: \"gha_${{github.repository}}_${{github.workflow}}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\"\n  cancel-in-progress: ${{github.ref != 'refs/heads/master'}}\njobs: {}\n",
		// A reusable workflow runs in the caller's context, so the same group deadlocks.
		"on: workflow_call\njobs: {}\n",
		"on:\n  workflow_call:\n  push:\njobs: {}\n",
		"runs:\n  using: composite\n",
	} {
		assert.Empty(t, concurrencyFindings(content), content)
	}
}

func TestTheRepairWritesTheOrgBlock(t *testing.T) {
	cases := map[string]string{
		"on: workflow_dispatch\njobs: {}\n":                                                        "on: workflow_dispatch\n" + orgBlock + "jobs: {}\n",
		"on: workflow_dispatch\n\njobs: {}\n":                                                      "on: workflow_dispatch\n\n" + orgBlock + "\njobs: {}\n",
		"on:\n  pull_request:\n# the jobs\njobs: {}\n":                                             "on:\n  pull_request:\n" + orgBlock + "# the jobs\njobs: {}\n",
		"on: workflow_dispatch\nconcurrency: ci\njobs: {}\n":                                       "on: workflow_dispatch\n" + orgBlock + "jobs: {}\n",
		"on: workflow_dispatch\nconcurrency:\n  group: ci\n  cancel-in-progress: true\njobs: {}\n": "on: workflow_dispatch\n" + orgBlock + "jobs: {}\n",
	}
	for content, want := range cases {
		out := workflow.Fix(content, func(string) bool { return true }).Text
		assert.Equal(t, want, out, content)
		assert.Empty(t, concurrencyFindings(out), content)
	}
}

func TestTheRepairKeepsTheFileIndent(t *testing.T) {
	content := "on:\n    workflow_dispatch:\njobs:\n    a:\n        runs-on: x\n"
	out := workflow.Fix(content, func(string) bool { return true }).Text
	assert.Contains(t, out, "concurrency:\n    group: gha_")
	assert.Contains(t, out, "\n    cancel-in-progress: ${{")
	assert.Empty(t, concurrencyFindings(out))
}
