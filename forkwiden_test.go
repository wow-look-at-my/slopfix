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
	req := slopfix.Request{Path: "p.go", Content: src, IDs: []string{"comments/number"}, MaxCommentLines: tombstones.DefaultMaxCommentLines}
	assert.Equal(t, src, slopfix.Fix(req).Text)

	req.IDs = []string{tombstones.IDVolume}
	assert.NotEqual(t, src, slopfix.Fix(req).Text, "the volume rule alone still cuts the block")
}

// upstreamLib opens with a crate doc longer than the code it documents.
const upstreamLib = "//! The crate holds the transport.\n//! It owns the client.\n//! It owns the tool calls.\n//! It owns the error classes.\n//! It owns the refresh.\n\npub mod servers;\n"

// forkLib adds a trailing count above that doc.
const forkLib = "#![allow(clippy::unwrap_used)] // 2 hits predate the gate\n" + upstreamLib

// The count is the fork's to repair, and the cut the length rule wants
// falls on the base's doc. The count lands, and the doc stays.
func TestAForkRepairLandsBesideAnUpstreamCutItMayNotMake(t *testing.T) {
	req := slopfix.Request{Path: "src/lib.rs", Content: forkLib, Owned: forkscope.Changed(upstreamLib, forkLib)}
	repair := slopfix.Fix(req)
	assert.Empty(t, repair.Findings)
	assert.NotContains(t, repair.Text, "2 hits", "the fork's count is repaired")
	assert.Contains(t, repair.Text, upstreamLib, "the base's doc stays as the base wrote it")
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
