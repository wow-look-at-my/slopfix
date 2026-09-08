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
)

// runLengthOn drives the command and returns what it printed. The command is
// built per call: tests run in parallel, and rootCmd's writer is shared.
func runLengthOn(t *testing.T, repair bool, paths ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.Flags().Bool("fix", repair, "")
	err := runCommentLength(cmd, paths)
	return out.String(), err
}

// essay is a comment far past anything a single declaration can carry.
func essay(marker string) string {
	var b strings.Builder
	for range 6 {
		b.WriteString(marker + " An explanation that runs well past the declaration below it.\n")
	}
	return b.String()
}

// A finding names its line and quotes its opening, or the reader searches the
// file for the block the rule meant.
func TestAnOverLongCommentIsReportedAndExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "a.go", "package p\n\n"+essay("//")+"const p = 1\n")

	out, err := runLengthOn(t, false, path)
	require.Error(t, err)
	assert.Contains(t, out, path+":3:")
	assert.Contains(t, out, "An explanation that runs well past")
}

// The repair is what makes this rule actionable. Reporting a finding nothing
// can act on is the failure the whole rule was rewritten to avoid.
func TestTheRepairShortensTheFileAndClearsTheFinding(t *testing.T) {
	dir := t.TempDir()
	body := "package p\n\n" + essay("//") + "const p = 1\n"
	path := writeAt(t, dir, "a.go", body)

	out, err := runLengthOn(t, true, path)
	require.NoError(t, err)
	assert.Contains(t, out, "repaired")

	fixed, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Less(t, len(fixed), len(body))
	assert.Contains(t, string(fixed), "An explanation that runs well past")

	after, err := runLengthOn(t, false, path)
	require.NoError(t, err)
	assert.Empty(t, after)
}

// The walk reads every grammar the rule claims, which is the reason it left a
// Go-only analyzer.
func TestTheWalkCrossesLanguages(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, dir, "a.go", "package p\n\n"+essay("//")+"const p = 1\n")
	writeAt(t, dir, "b.rs", essay("//")+"const P: i32 = 1;\n")
	writeAt(t, dir, "c.sh", "#!/bin/sh\n"+essay("#")+"p=1\n")
	writeAt(t, dir, "notes.md", essay("//"))

	out, err := runLengthOn(t, false, dir)
	require.Error(t, err)
	for _, name := range []string{"a.go", "b.rs", "c.sh"} {
		assert.Contains(t, out, name)
	}
	assert.NotContains(t, out, "notes.md", "the rule has no grammar for markdown")
}

// A proportionate comment says nothing and succeeds, or the command is noise.
func TestAProportionateCommentSaysNothing(t *testing.T) {
	dir := t.TempDir()
	path := writeAt(t, dir, "a.go", "package p\n\n// The bound every caller shares.\nconst p = 1\n")

	out, err := runLengthOn(t, false, path)
	require.NoError(t, err)
	assert.Empty(t, out)
}

// A path that does not exist is an error, never a silent pass.
func TestAMissingPathIsAnErrorForLength(t *testing.T) {
	_, err := runLengthOn(t, false, filepath.Join(t.TempDir(), "absent.go"))
	assert.Error(t, err)
}
