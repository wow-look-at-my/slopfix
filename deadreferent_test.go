package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// deadReferentRepo answers a path for name in a repository whose only other
// file defines nothing the documents below name.
func deadReferentRepo(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "-q").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n\nfunc readInput() {}\n"), 0o644))
	return filepath.Join(dir, name)
}

// A sentence that wraps onto the line with the dead name is cut whole, and the
// period on the line above stays.
func TestFixCutsADeadNameFromAWrappedSentence(t *testing.T) {
	path := deadReferentRepo(t, "p.go")
	content := "package p\n\nfunc f() {\n\t// A tag-excluded file is never compiled, so it is not coverable. A match\n\t// error means it cannot classify; include it (see fileMatchesBuild).\n\tf()\n}\n"
	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.Equal(t, "package p\n\nfunc f() {\n\t// A tag-excluded file is never compiled, so it is not coverable.\n\tf()\n}\n", fixed.Text)
}

// A sentence that runs from the dead name onto the lines below is cut whole.
// When it fills the comment, the comment goes.
func TestFixCutsADeadNameWhoseSentenceRunsDown(t *testing.T) {
	path := deadReferentRepo(t, "p.go")
	content := "package p\n\n// sgArmAux records every arm's cost and its rate relative to the\n// measured rate. SgArmAux returns the per-arm map the\n// caller's ratios read.\nfunc sgArmAux() {}\n"
	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.Equal(t, "package p\n\n// sgArmAux records every arm's cost and its rate relative to the measured rate.\nfunc sgArmAux() {}\n", fixed.Text)

	whole := "package p\n\nfunc f() {}\n\n// errNoInst is a placeholder for an error raised\n// outside any single instruction.\n\nvar x = 1\n"
	fixed = slopfix.Fix(slopfix.Request{Path: path, Content: whole})
	assert.Equal(t, "package p\n\nfunc f() {}\n\nvar x = 1\n", fixed.Text)
}

// A cut that opens a comment and stops mid-line leaves one blank after the
// marker, which is what gofmt keeps.
func TestFixLeavesOneBlankAfterTheMarker(t *testing.T) {
	path := deadReferentRepo(t, "p.go")
	content := "package p\n\n// fdinfoShape is the shape the kernel emits (see\n// amdgpu_show_fdinfo): identity, then timers. The\n// gfx-only pattern is what the tool produces.\nconst fdinfoShape = 1\n"
	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.NotContains(t, fixed.Text, "//  ")
	assert.Contains(t, fixed.Text, "The gfx-only pattern is what the tool produces.")
	assert.NotContains(t, fixed.Text, "amdgpu_show_fdinfo")
}

// The marker of a comment is its first one. A `//` inside a URL is text, so
// the cut keeps the sentence before the dead name.
func TestFixKeepsTheTextBeforeAURL(t *testing.T) {
	path := deadReferentRepo(t, "p.go")
	content := "package p\n\n// WriteChrome writes the events as a\n// trace file. Load it in chrome://tracing or the ProfilerPanel tab.\nfunc WriteChrome() {}\n"
	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.Equal(t, "package p\n\n// WriteChrome writes the events as a trace file.\nfunc WriteChrome() {}\n", fixed.Text)
}

// A document that names a symbol nothing defines is reported on the line that
// names it, and fix cuts the sentence.
func TestFixCutsADeadNameFromADocument(t *testing.T) {
	path := deadReferentRepo(t, "x.md")
	content := "# Notes\n\nThe APE is a cosmo build everywhere, so claudeguard_proc.go is the classifier on NT too. It reads /proc, which NT does not have.\n"

	report := slopfix.Report(slopfix.Request{Path: path, Content: content})
	var lines []int
	for _, h := range report.Kept {
		if h.ID == tombstones.IDDeadReferent {
			lines = append(lines, h.LineNo)
		}
	}
	assert.Equal(t, []int{3}, lines, "check names the line that holds the name, counted from one")

	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.NotContains(t, fixed.Text, "claudeguard_proc")
	assert.Contains(t, fixed.Text, "It reads /proc, which NT does not have.")
	assert.NotContains(t, keptIDs(fixed.Kept), tombstones.IDDeadReferent)
}

// A comment line that names nothing the repository defines is one fix strips
// whole. Check strips nothing, so it reports the line instead.
func TestCheckReportsTheDeadNameFixStrips(t *testing.T) {
	path := deadReferentRepo(t, "main.go")
	content := "package main\n\n// parseLegacyFlag reads the input.\nfunc main() {}\n"

	report := slopfix.Report(slopfix.Request{Path: path, Content: content})
	assert.Contains(t, keptIDs(report.Kept), tombstones.IDDeadReferent)

	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.NotContains(t, fixed.Text, "parseLegacyFlag")
	assert.NotContains(t, keptIDs(fixed.Kept), tombstones.IDDeadReferent)
}
