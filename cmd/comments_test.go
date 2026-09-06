package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCommentsOn drives the command and returns what it printed. The command is
// built per call: tests run in parallel, and rootCmd's writer is shared.
func runCommentsOn(t *testing.T, paths ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
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
