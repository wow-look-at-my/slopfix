package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/commentnumbers"
)

// runCommentsOn drives the command and returns what it printed.
func runCommentsOn(t *testing.T, paths ...string) (string, error) {
	t.Helper()
	return runCommentsFix(t, false, paths...)
}

// runCommentsFix is the same, with the repair flag the command reads.
func runCommentsFix(t *testing.T, repair bool, paths ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.Flags().Bool("fix", repair, "")
	err := runComments(cmd, paths)
	return out.String(), err
}

// writeAt puts a file under dir and returns its path.
func writeAt(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

// A finding has to name where it sits, or the reader searches the file.
func TestAFindingNamesItsPlaceAndExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "a.go", "package p\n\n// holds 3 entries\n")

	out, err := runCommentsOn(t, path)
	require.Error(t, err)
	assert.Contains(t, out, path+":3:10:")
	assert.Contains(t, out, `"3" is a number in a comment`)
	assert.Contains(t, out, "let the reader count")
}

// Clean prose says nothing and succeeds, or the command is noise.
func TestCleanProseSaysNothing(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "a.go", "package p\n\n// holds the entries\n")

	out, err := runCommentsOn(t, path)
	require.NoError(t, err)
	assert.Empty(t, out)
}

// A directory is walked, across languages, which is the reason this rule left
// a Go-only analyzer.
func TestADirectoryIsWalkedAcrossLanguages(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "a.go", "package p\n\n// the walk has 3 phases\n")
	writeAt(t, dir, "run.sh", "#!/bin/sh\n# the sweep runs twice\n")
	writeAt(t, dir, "ci.yml", "# holds 4 jobs\njobs: {}\n")

	out, err := runCommentsOn(t, dir)
	require.Error(t, err)
	assert.Contains(t, out, `"3"`)
	assert.Contains(t, out, `"twice"`)
	assert.Contains(t, out, `"4"`)
}

// A walk skips what nobody in the tree authored.
func TestTheWalkSkipsForeignText(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "vendor/dep/a.go", "package p\n\n// holds 3 entries\n")
	writeAt(t, dir, ".git/hooks/h.sh", "# runs once\n")

	out, err := runCommentsOn(t, dir)
	require.NoError(t, err)
	assert.Empty(t, out)
}

// Naming a file IS the request, so its extension does not veto it.
func TestANamedFileIsReadWhateverItsExtension(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "Dockerfile", "# runs once\nFROM scratch\n")

	out, err := runCommentsOn(t, path)
	require.Error(t, err)
	assert.Contains(t, out, `"once"`)
}

// A path that does not exist is an error, never a silent pass.
func TestAMissingPathIsAnError(t *testing.T) {
	_, err := runCommentsOn(t, filepath.Join(t.TempDir(), "absent.go"))
	assert.Error(t, err)
}

// The rule had a repair the whole time and no way to reach it, so every finding
// was somebody's hand edit. The table says the number in words where it can.
func TestFixSaysTheNumberInWords(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "a.go", "package p\n\n// Asked once per repository.\nconst p = 1\n")

	out, err := runCommentsFix(t, true, path)
	require.NoError(t, err, "nothing is left to fail over")
	assert.Contains(t, out, "repaired")

	body, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "a single time per repository")
	assert.NotContains(t, string(body), "once")
}

// What the table does not cover is cut, and the cut sentence is printed: the
// file no longer holds it, so this output is the only record.
func TestFixCutsWhatTheTableCannotSayAndPrintsIt(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "a.go", "package p\n\n// Keeps the state. The tables run to 12 sections.\nconst p = 1\n")

	out, err := runCommentsFix(t, true, path)
	require.NoError(t, err)
	assert.Contains(t, out, "cut: The tables run to 12 sections.")

	body, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "Keeps the state.", "the sentence carrying no number stays")
	assert.NotContains(t, string(body), "12")
}

func TestFixStillFailsOnWhatItCouldNotRepair(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "a.go", "package p\n\nconst p = 1 // 12\n")

	_, err := runCommentsFix(t, true, path)
	if len(commentNumbersLeft(t, path)) > 0 {
		assert.Error(t, err)
		return
	}
	assert.NoError(t, err)
}

// commentNumbersLeft re-reads the file through the rule, so the assertion above
// tracks the repair rather than restating today's outcome.
func commentNumbersLeft(t *testing.T, path string) []commentnumbers.Hit {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return commentnumbers.Check(path, string(body))
}

// A submodule is another repository's checkout, and its prose is that
// repository's to fix. The binary and the pipeline gate both skip it, or a
// developer and CI disagree about what the tree contains.
func TestASubmoduleIsNotWalked(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "ours.go", "package p\n\n// Asked once.\nconst p = 1\n")
	writeAt(t, dir, filepath.Join("upstream", ".git"), "gitdir: ../.git/modules/upstream\n")
	writeAt(t, dir, filepath.Join("upstream", "theirs.c"), "/* runs once */\n")

	paths, err := commentTargets(dir, commentnumbers.Supported)
	require.NoError(t, err)
	joined := strings.Join(paths, "\n")
	assert.Contains(t, joined, "ours.go")
	assert.NotContains(t, joined, "theirs.c")
}
