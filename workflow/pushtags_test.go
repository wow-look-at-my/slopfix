package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

func pushTagFindings(content string) []ste.Finding {
	var out []ste.Finding
	for _, f := range workflow.Check(content) {
		if f.ID == workflow.IDPushTags {
			out = append(out, f)
		}
	}
	return out
}

func TestAnUnfilteredPushIsReported(t *testing.T) {
	cases := map[string]int{
		"on: push\njobs: {}\n":                                     1,
		"on: [pull_request, push]\njobs: {}\n":                     1,
		"on:\n  push:\n  workflow_dispatch:\njobs: {}\n":           2,
		"on:\n  push:\n    paths: ['src/**']\njobs: {}\n":          2,
		"'on':\n  push: {}\njobs: {}\n":                            2,
		"name: ci\non:\n  workflow_dispatch:\n  push:\njobs: {}\n": 4,
	}
	for content, line := range cases {
		found := pushTagFindings(content)
		require.Len(t, found, 1, content)
		assert.Equal(t, line, found[0].Line, content)
		assert.Contains(t, found[0].Fix, "branches: ['**']", content)
	}
}

func TestTheRepairWritesABranchFilter(t *testing.T) {
	cases := map[string]string{
		"on: push\njobs: {}\n":                                  "on:\n  push:\n    branches: ['**']\njobs: {}\n",
		"on: push # ci\njobs: {}\n":                             "on: # ci\n  push:\n    branches: ['**']\njobs: {}\n",
		"on: [pull_request, push]\njobs: {}\n":                  "on:\n  pull_request:\n  push:\n    branches: ['**']\njobs: {}\n",
		"on:\n  push:\n  workflow_dispatch:\njobs: {}\n":        "on:\n  push:\n    branches: ['**']\n  workflow_dispatch:\njobs: {}\n",
		"on:\n    push:\n        paths: ['src/**']\njobs: {}\n": "on:\n    push:\n        branches: ['**']\n        paths: ['src/**']\njobs: {}\n",
	}
	for content, want := range cases {
		out := repaired(t, content)
		assert.Equal(t, want, out, content)
		assert.Empty(t, pushTagFindings(out), content)
	}
}

// A shape no line edit reaches gets the whole on: value again, in block style.
func TestAFlowStylePushIsWrittenAgainInBlockStyle(t *testing.T) {
	cases := map[string]string{
		"'on':\n  push: {}\njobs: {}\n":               "'on':\n  push:\n    branches: ['**']\njobs: {}\n",
		"on: {push: {paths: [a]}}\njobs: {}\n":        "on:\n  push:\n    paths: [a]\n    branches: ['**']\njobs: {}\n",
		"on:\n  - push\n  - pull_request\njobs: {}\n": "on:\n  push:\n    branches: ['**']\n  pull_request: null\njobs: {}\n",
		"on:\n  push: {}\n\n# the jobs\njobs: {}\n":   "on:\n  push:\n    branches: ['**']\n\n# the jobs\njobs: {}\n",
	}
	for content, want := range cases {
		out := repaired(t, content)
		assert.Equal(t, want, out, content)
		assert.Empty(t, pushTagFindings(out), content)
	}
}

func TestAFilteredPushPasses(t *testing.T) {
	for _, content := range []string{
		"on:\n  push:\n    branches: ['**']\njobs: {}\n",
		"on:\n  push:\n    branches-ignore: [gh-pages]\njobs: {}\n",
		"on:\n  push:\n    tags: ['v*']\njobs: {}\n",
		"on:\n  push:\n    tags-ignore: ['**']\njobs: {}\n",
		"on: workflow_dispatch\njobs: {}\n",
		"on: [pull_request]\njobs: {}\n",
		"runs:\n  using: composite\n",
	} {
		assert.Empty(t, pushTagFindings(content), content)
	}
}
