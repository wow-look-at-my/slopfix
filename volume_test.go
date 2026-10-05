package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// A block over the volume cap sits above code long enough that the length rule
// alone never cuts it. The fix still brings it under the cap.
func TestFixCutsABlockBackUnderTheVolumeCap(t *testing.T) {
	var src strings.Builder
	src.WriteString("package p\n\n")
	for range 20 {
		src.WriteString("// Run reads the queue and writes each entry out, then it waits for the next one to come in.\n")
	}
	src.WriteString("func Run() {\n")
	for range 40 {
		src.WriteString("\tstep(theFirstArgumentOfTheCall, theSecondArgumentOfTheCall, theThirdArgument)\n")
	}
	src.WriteString("}\n")
	assert.Empty(t, commentfix.CheckLength("a.go", src.String()), "only the volume cap judges this block")

	repair := slopfix.Fix(slopfix.Request{Content: src.String(), Path: "a.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	comments := 0
	for _, line := range strings.Split(repair.Text, "\n") {
		if strings.HasPrefix(line, "//") {
			comments++
		}
	}
	assert.LessOrEqual(t, comments, tombstones.DefaultMaxCommentLines, repair.Text)
	assert.Contains(t, repair.Text, "func Run() {\n\tstep(theFirstArgumentOfTheCall", "the code is untouched")
}

// Each field doc of a Rust struct is its own block, so short docs on many fields never add up to a volume finding.
func TestRustFieldDocsAreSeparateBlocks(t *testing.T) {
	var src strings.Builder
	src.WriteString("pub struct Tracker {\n")
	for i := range 10 {
		src.WriteString("    /// Entry that receives the deltas of this stream.\n")
		src.WriteString("    /// None between turns, and before the first chunk.\n")
		src.WriteString("    field_" + string(rune('a'+i)) + ": Option<u64>,\n")
	}
	src.WriteString("}\n")

	repair := slopfix.Report(slopfix.Request{Path: "tracker.rs", Content: src.String(), MaxCommentLines: tombstones.DefaultMaxCommentLines})
	for _, k := range repair.Kept {
		assert.NotEqual(t, tombstones.IDVolume, k.ID, "line %d: %s", k.LineNo, k.Tell)
	}
}

// A comment on a line of code is not part of the doc block below it, so the block starts at its own first line.
func TestATrailingCommentIsNotPartOfTheBlockBelow(t *testing.T) {
	var src strings.Builder
	src.WriteString("#![allow(clippy::expect_used)] // Hits predate the gate\n")
	src.WriteString("#![allow(clippy::unwrap_used)] // Hits predate the gate\n")
	for range 20 {
		src.WriteString("//! The crate reads the queue and writes each entry out.\n")
	}
	src.WriteString("\npub fn run() {}\n")

	repair := slopfix.Report(slopfix.Request{Path: "lib.rs", Content: src.String(), MaxCommentLines: tombstones.DefaultMaxCommentLines})
	var starts []int
	for _, k := range repair.Kept {
		if k.ID == tombstones.IDVolume {
			starts = append(starts, k.LineNo)
		}
	}
	assert.Equal(t, []int{3}, starts, "the block is the doc alone")
}

// A package doc has no construct to weigh against, so only the cap judges it.
// The fix cuts it the same way and keeps the package clause.
func TestFixCutsAPackageDocBackUnderTheVolumeCap(t *testing.T) {
	var src strings.Builder
	src.WriteString("//go:build linux\n\n")
	for range 10 {
		src.WriteString("// Package p reads the queue and writes each entry out. It waits for the next one.\n")
		src.WriteString("//\n")
	}
	src.WriteString("package p\n\nfunc Run() {}\n")
	assert.Empty(t, commentfix.CheckLength("a.go", src.String()), "the length rule never judges a package doc")

	repair := slopfix.Fix(slopfix.Request{Content: src.String(), Path: "a.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	comments := 0
	for _, line := range strings.Split(repair.Text, "\n") {
		if strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "//go:") {
			comments++
		}
	}
	assert.LessOrEqual(t, comments, tombstones.DefaultMaxCommentLines, repair.Text)
	assert.Contains(t, repair.Text, "//go:build linux\n", "the constraint stays")
	assert.Contains(t, repair.Text, "\npackage p\n\nfunc Run() {}\n", "the code is untouched")
	assert.Empty(t, slopfix.Fix(slopfix.Request{Content: repair.Text, Path: "a.go", MaxCommentLines: tombstones.DefaultMaxCommentLines}).Kept, "nothing is left to report")
}
