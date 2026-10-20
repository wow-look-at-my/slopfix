package workflow_test

import _ "embed"

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
		"on: push\njobs: {}\n":                 1,
		"on: [pull_request, push]\njobs: {}\n": 1,
		testAnUnfilteredPushIsReportedYml:      2,
		testAnUnfilteredPushIsReportedYml2:     2,
		testAnUnfilteredPushIsReportedYml3:     2,
		testAnUnfilteredPushIsReportedYml4:     4,
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
		"on: push\njobs: {}\n":                 testTheRepairWritesABranchFilterYml,
		"on: push # ci\njobs: {}\n":            testTheRepairWritesABranchFilterYml2,
		"on: [pull_request, push]\njobs: {}\n": testTheRepairWritesABranchFilterYml3,
		testTheRepairWritesABranchFilterYml4:   testTheRepairWritesABranchFilterYml5,
		testTheRepairWritesABranchFilterYml6:   testTheRepairWritesABranchFilterYml7,
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
		testAFlowStylePushIsWrittenAgainInBlockStyleYml:  testAFlowStylePushIsWrittenAgainInBlockStyleYml2,
		"on: {push: {paths: [a]}}\njobs: {}\n":           testAFlowStylePushIsWrittenAgainInBlockStyleYml3,
		testAFlowStylePushIsWrittenAgainInBlockStyleYml4: testAFlowStylePushIsWrittenAgainInBlockStyleYml5,
		testAFlowStylePushIsWrittenAgainInBlockStyleYml6: testAFlowStylePushIsWrittenAgainInBlockStyleYml7,
	}
	for content, want := range cases {
		out := repaired(t, content)
		assert.Equal(t, want, out, content)
		assert.Empty(t, pushTagFindings(out), content)
	}
}

func TestAFilteredPushPasses(t *testing.T) {
	for _, content := range []string{
		testAFilteredPushPassesYml,
		testAFilteredPushPassesYml2,
		testAFilteredPushPassesYml3,
		testAFilteredPushPassesYml4,
		"on: workflow_dispatch\njobs: {}\n",
		"on: [pull_request]\njobs: {}\n",
		"runs:\n  using: composite\n",
	} {
		assert.Empty(t, pushTagFindings(content), content)
	}
}

//go:embed testdata/test-an-unfiltered-push-is-reported.yml
var testAnUnfilteredPushIsReportedYml string

//go:embed testdata/test-an-unfiltered-push-is-reported-2.yml
var testAnUnfilteredPushIsReportedYml2 string

//go:embed testdata/test-an-unfiltered-push-is-reported-3.yml
var testAnUnfilteredPushIsReportedYml3 string

//go:embed testdata/test-an-unfiltered-push-is-reported-4.yml
var testAnUnfilteredPushIsReportedYml4 string

//go:embed testdata/test-the-repair-writes-a-branch-filter.yml
var testTheRepairWritesABranchFilterYml string

//go:embed testdata/test-the-repair-writes-a-branch-filter-2.yml
var testTheRepairWritesABranchFilterYml2 string

//go:embed testdata/test-the-repair-writes-a-branch-filter-3.yml
var testTheRepairWritesABranchFilterYml3 string

//go:embed testdata/test-the-repair-writes-a-branch-filter-4.yml
var testTheRepairWritesABranchFilterYml4 string

//go:embed testdata/test-the-repair-writes-a-branch-filter-5.yml
var testTheRepairWritesABranchFilterYml5 string

//go:embed testdata/test-the-repair-writes-a-branch-filter-6.yml
var testTheRepairWritesABranchFilterYml6 string

//go:embed testdata/test-the-repair-writes-a-branch-filter-7.yml
var testTheRepairWritesABranchFilterYml7 string

//go:embed testdata/test-a-flow-style-push-is-written-again-in-block-style.yml
var testAFlowStylePushIsWrittenAgainInBlockStyleYml string

//go:embed testdata/test-a-flow-style-push-is-written-again-in-block-style-2.yml
var testAFlowStylePushIsWrittenAgainInBlockStyleYml2 string

//go:embed testdata/test-a-flow-style-push-is-written-again-in-block-style-3.yml
var testAFlowStylePushIsWrittenAgainInBlockStyleYml3 string

//go:embed testdata/test-a-flow-style-push-is-written-again-in-block-style-4.yml
var testAFlowStylePushIsWrittenAgainInBlockStyleYml4 string

//go:embed testdata/test-a-flow-style-push-is-written-again-in-block-style-5.yml
var testAFlowStylePushIsWrittenAgainInBlockStyleYml5 string

//go:embed testdata/test-a-flow-style-push-is-written-again-in-block-style-6.yml
var testAFlowStylePushIsWrittenAgainInBlockStyleYml6 string

//go:embed testdata/test-a-flow-style-push-is-written-again-in-block-style-7.yml
var testAFlowStylePushIsWrittenAgainInBlockStyleYml7 string

//go:embed testdata/test-a-filtered-push-passes.yml
var testAFilteredPushPassesYml string

//go:embed testdata/test-a-filtered-push-passes-2.yml
var testAFilteredPushPassesYml2 string

//go:embed testdata/test-a-filtered-push-passes-3.yml
var testAFilteredPushPassesYml3 string

//go:embed testdata/test-a-filtered-push-passes-4.yml
var testAFilteredPushPassesYml4 string
