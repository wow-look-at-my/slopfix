package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/forkscope"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// upstreamReadme holds a paragraph of one line, as the base has it.
const upstreamReadme = "# Tool\n\nThe tool reads the file.\n\nIt writes the result.\n"

// forkReadme adds lines to that paragraph. The paragraph is now hard-wrapped, and its first line is still the base's.
const forkReadme = "# Tool\n\nThe tool reads the file.\nIt then checks the file\nand reports each error.\n\nIt writes the result.\n"

// A run that leaves the volume rule out never cuts a comment block to the cap.
func TestTheVolumeCutNeedsTheVolumeRule(t *testing.T) {
	src := "package p\n\n" + strings.Repeat("// The value is read once.\n", 20) + "var x = 1\n"
	req := slopfix.Request{Path: "p.go", Content: src, IDs: []string{"tombstones/date"}, MaxCommentLines: tombstones.DefaultMaxCommentLines}
	assert.Equal(t, src, slopfix.Fix(req).Text)

	req.IDs = []string{tombstones.IDVolume}
	assert.NotEqual(t, src, slopfix.Fix(req).Text, "the volume rule alone still cuts the block")
}

// A fork that writes lines into a paragraph owns the paragraph. A rule that
// judges the paragraph whole can then repair it, the base's line too.
func TestAForkOwnsEachParagraphItWroteInto(t *testing.T) {
	owned := forkscope.Changed(upstreamReadme, forkReadme)
	req := slopfix.Request{Path: "README.md", Content: forkReadme, Owned: owned}

	report := slopfix.Report(req)
	assert.Contains(t, repairIDs(report), slopfix.IDHardWrap, "the fork wrapped the paragraph")

	repair := slopfix.Fix(req)
	assert.Empty(t, repair.Findings, "the fix reaches the base's line of the paragraph")
	assert.Contains(t, repair.Text, "The tool reads the file. It then checks the file and reports each error.\n")
	assert.Contains(t, repair.Text, "\nIt writes the result.\n", "a paragraph the fork never touched stays")
}
