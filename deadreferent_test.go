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

// A sentence that names a dead symbol goes whole, across every line it wraps
// onto, and the comment keeps its other sentences.
func TestFixCutsTheWholeSentenceThatWrapsADeadName(t *testing.T) {
	path := deadReferentRepo(t, "main.go")
	content := "package main\n\n// main reads the input. The check in\n// parseLegacyFlag runs first, so the caller\n// waits for it.\nfunc main() {}\n"

	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.Equal(t, "package main\n\n// main reads the input.\nfunc main() {}\n", fixed.Text)
}

// A name in the code before a trailing comment is no name the comment carries.
func TestADeadNameRuleReadsOnlyTheComment(t *testing.T) {
	path := deadReferentRepo(t, "main.go")
	content := "package main\n\nvar names = []string{\n\t\"wasmonly_wasm_js\", // js-only main\n}\n"

	report := slopfix.Report(slopfix.Request{Path: path, Content: content})
	assert.NotContains(t, keptIDs(report.Kept), tombstones.IDDeadReferent)
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
