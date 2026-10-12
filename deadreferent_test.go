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

// A dead name that differs from a defined one in case alone is a typo. Fix
// renames it and keeps the line, which holds the opening sentence too.
func TestFixRenamesACaseTypoToTheDefinedName(t *testing.T) {
	path := deadReferentRepo(t, "os_cosmo_nt_msg.go")
	content := "package runtime\n\n" +
		"// ntSCMParse walks a Linux cmsg buffer. NtSCMParse collects the SCM_RIGHTS\n" +
		"// payload fds, with the kernel's exact acceptance rules.\n" +
		"func ntSCMParse() {}\n"

	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.NotContains(t, fixed.Text, "NtSCMParse")
	assert.Contains(t, fixed.Text, "// ntSCMParse walks a Linux cmsg buffer. ntSCMParse collects the SCM_RIGHTS\n// payload fds")
	assert.NotContains(t, keptIDs(fixed.Kept), tombstones.IDDeadReferent)
}

// A doc comment whose subject is stale names the declaration below it. Fix
// renames the subject to that declaration and keeps the comment.
func TestFixRenamesAStaleSubjectToTheDocumentedField(t *testing.T) {
	path := deadReferentRepo(t, "github_test.go")
	content := "package codehost\n\n" +
		"type fakeServer struct {\n" +
		"\t// lastInfoRefsCredential is the Authorization of every info/refs request, in order.\n" +
		"\tinfoRefsAuth []string\n" +
		"}\n"

	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.NotContains(t, fixed.Text, "lastInfoRefsCredential")
	assert.Contains(t, fixed.Text, "\t// infoRefsAuth is the Authorization of every info/refs request")
	assert.NotContains(t, keptIDs(fixed.Kept), tombstones.IDDeadReferent)
}

// A dead name with no defined name to take its place loses its sentence alone,
// across lines, and the sentences around it stay.
func TestFixCutsOnlyTheSentenceThatHoldsADeadName(t *testing.T) {
	path := deadReferentRepo(t, "walk.go")
	content := "package p\n\n" +
		"// readInput walks the buffer. GoneHelperName collects the payload\n" +
		"// with the exact rules. It returns the count.\n" +
		"func readInput() {}\n"

	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.NotContains(t, fixed.Text, "GoneHelperName")
	assert.Contains(t, fixed.Text, "// readInput walks the buffer. It returns the count.\n")
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
