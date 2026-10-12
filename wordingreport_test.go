package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// keptIDs answers the rule of each hit a run kept.
func keptIDs(hits []tombstones.Hit) []string {
	var ids []string
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	return ids
}

// A wording rule that fix repairs is a finding check reports, in a document
// and in a comment. Check writes nothing, so the phrase is still there to name.
func TestCheckReportsTheWordingFixCuts(t *testing.T) {
	for path, content := range map[string]string{
		"x.md": "For now it retries once.\n",
		"x.go": "package x\n\n// For now it retries once.\nfunc f() {}\n",
	} {
		report := slopfix.Report(slopfix.Request{Path: path, Content: content})
		assert.Contains(t, keptIDs(report.Kept), "tombstones/hedged-time", path)

		fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
		assert.NotContains(t, fixed.Text, "For now", path)
		assert.NotContains(t, keptIDs(fixed.Kept), "tombstones/hedged-time", path)
	}
}

// The control: text with no wording to cut reports none.
func TestCheckReportsNoWordingOnPlainText(t *testing.T) {
	report := slopfix.Report(slopfix.Request{Path: "x.md", Content: "It retries once.\n"})
	assert.NotContains(t, keptIDs(report.Kept), "tombstones/hedged-time")
}
