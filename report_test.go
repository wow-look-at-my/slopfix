package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
)

// A check judges the text as written. A block the repair would cut is a
// finding, though nothing is left of it after the repair.
func TestReportNamesWhatTheRepairWouldChange(t *testing.T) {
	src := "package p\n\n" +
		"// FullMipChain marks a TextureBinding as carrying its complete mip pyramid\n" +
		"// when the exact level count is not supplied, which the caller signals.\n" +
		"const FullMipChain = -1\n"
	repair := slopfix.Report(slopfix.Request{Path: "p.go", Content: src, Rules: []slopfix.Rule{slopfix.RuleComments}})
	var ids []string
	for _, f := range repair.Findings {
		ids = append(ids, f.ID)
	}
	for _, k := range repair.Kept {
		ids = append(ids, k.ID)
	}
	assert.Contains(t, ids, "comments/length")
}

// A rule the caller did not select reports nothing, in a workflow as anywhere.
func TestASelectionLeavesWorkflowRulesOut(t *testing.T) {
	src := "name: CI\non:\n  push:\njobs:\n  test:\n    runs-on: ubuntu-latest\n" +
		"    steps:\n      # one\n      # two\n      # three\n      - run: make\n"
	repair := slopfix.Report(slopfix.Request{Path: ".github/workflows/ci.yml", Content: src, Rules: []slopfix.Rule{slopfix.RuleComments}})
	for _, f := range repair.Findings {
		assert.False(t, strings.HasPrefix(f.ID, "yaml/"), "an unselected rule reported: %s", f)
	}
}
