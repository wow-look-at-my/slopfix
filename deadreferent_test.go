package slopfix_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// gl-dsa-shim names OpenGL extensions that its OpenGL-Registry submodule
// defines. With that submodule not checked out, no probe can read them, so no
// name is reported dead.
func TestANameFromASubmoduleWithNoCheckoutIsNotDead(t *testing.T) {
	path := deadReferentRepo(t, "shim_gl_4_3.cpp")
	dir := filepath.Dir(path)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "third_party", "OpenGL-Registry"), 0o755))
	require.NoError(t, exec.Command("git", "-C", dir, "update-index", "--add", "--cacheinfo",
		"160000,0123456789abcdef0123456789abcdef01234567,third_party/OpenGL-Registry").Run())
	content := "#include \"shim_internal.h\"\n\n" +
		"// macOS supports ARB_debug_output in some drivers. We emulate the KHR_debug interface.\n" +
		"static int g_debug = 0;\n\n" +
		"// Requires GL_ARB_vertex_attrib_binding, stub.\n" +
		"static int g_binding = 0;\n"

	report := slopfix.Report(slopfix.Request{Path: path, Content: content})
	assert.NotContains(t, keptIDs(report.Kept), tombstones.IDDeadReferent)
	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content}).Text
	assert.Contains(t, fixed, "KHR_debug")
	assert.Contains(t, fixed, "GL_ARB_vertex_attrib_binding")
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
		"func ntSCMParse() {\n" + strings.Repeat("\tstep()\n", 8) + "}\n"

	fixed := slopfix.Fix(slopfix.Request{Path: path, Content: content})
	assert.NotContains(t, fixed.Text, "NtSCMParse")
	assert.Contains(t, fixed.Text, "// ntSCMParse walks a Linux cmsg buffer. ntSCMParse collects the SCM_RIGHTS")
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
