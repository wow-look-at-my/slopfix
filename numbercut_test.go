package slopfix_test

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// An exit status is a value the program answers with, not a count of what
// exists here. No rule reports it, and the repair writes the comment back as
// the author wrote it.
func TestAnExitCodeInADocCommentIsNoCount(t *testing.T) {
	const doc = "/// Dispatch the observe-only session-end `Stop`: runs in stop-gate mode so\n" +
		"/// exit code 2 parses as a block, but the decision is discarded (no turn\n" +
		"/// left to continue).\n" +
		"pub(crate) async fn dispatch_session_end_stop(&self, reason: &str) {\n" +
		"    if self.startup_hints.is_subagent {\n" +
		"        return;\n" +
		"    }\n" +
		"}\n"
	for _, f := range slopfix.CheckContent("stop.rs", doc) {
		assert.NotEqual(t, commentfix.ID, f.ID, f.String())
	}
	got := slopfix.Fix(slopfix.Request{Content: doc, Path: "stop.rs", IDs: []string{commentfix.ID}, MaxCommentLines: tombstones.DefaultMaxCommentLines})
	assert.Equal(t, doc, got.Text)
}

//go:embed testdata/aligned-block.js
var alignedBlock string

// The count goes and the sentence still ends on its words, with the period on
// the line it closes.
func TestACountCutFromAnAlignedBlockLeavesNoLonePeriod(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{Content: alignedBlock, Path: "reveal.ts", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	comment, _, found := strings.Cut(repair.Text, "  function visit")
	assert.True(t, found, repair.Text)
	for _, line := range strings.Split(comment, "\n") {
		trimmed := strings.TrimSpace(line)
		assert.False(t, strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, ","), "a line opens on a mark:\n%s", repair.Text)
	}
	assert.NotContains(t, repair.Text, "two", repair.Text)
	assert.Contains(t, repair.Text, "miss those.", "the sentence ends on its words:\n%s", repair.Text)
	assert.Equal(t, 1, strings.Count(repair.Text, "*/"), repair.Text)
	assert.Contains(t, repair.Text, "  function visit(el: Element): void {\n", "the code is untouched")
	assert.Empty(t, commentfix.Check("reveal.ts", repair.Text), repair.Text)
}
