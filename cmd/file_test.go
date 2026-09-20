package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// drive runs a single subcommand of `file` with its own flag state, so a test
// never reads what a sibling set.
func drive(t *testing.T, run func(*cobra.Command, []string) error, stdin string, args ...string) (string, string, error) {
	t.Helper()
	t.Cleanup(func() {
		fileJSON, fileOnly, filePath, fileMaxLines = false, nil, "", 0
	})

	var out, errs bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&errs)
	cmd.SetIn(strings.NewReader(stdin))
	err := run(cmd, args)
	return out.String(), errs.String(), err
}

func put(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func body(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// A workflow and a document reach different rule families through a single
// command, which is the whole point of the merge.
func TestCheckReadsAWorkflowByItsPath(t *testing.T) {
	path := put(t, ".github/workflows/ci.yml", "name: CI\n# one\n# two\non: push\n")

	_, _, err := drive(t, runFileCheck, "", path)

	require.ErrorIs(t, err, errFindings)
}

func TestCheckReadsADocumentByItsPath(t *testing.T) {
	path := put(t, "doc.md", "This shouldn't run; it is banned.\n")

	_, _, err := drive(t, runFileCheck, "", path)

	require.ErrorIs(t, err, errFindings)
}

func TestACleanFileIsNoFinding(t *testing.T) {
	path := put(t, ".github/workflows/ci.yml", "name: CI\n# one\non: push\n")

	_, _, err := drive(t, runFileCheck, "", path)

	require.NoError(t, err)
}

func TestCheckAnswersAsJSON(t *testing.T) {
	t.Serial()
	path := put(t, ".github/workflows/ci.yml", "name: CI\n# one\n# two\non: push\n")
	fileJSON = true

	out, _, err := drive(t, runFileCheck, "", path)

	require.ErrorIs(t, err, errFindings)
	var got reportOutput
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Len(t, got.Findings, 1)
	assert.Equal(t, "yaml/comment-block", got.Findings[0].ID)
	assert.True(t, got.Findings[0].Repairable)
}

// The same file, repaired rather than reported.
func TestFixRepairsTheFileInPlace(t *testing.T) {
	path := put(t, ".github/workflows/ci.yml", "name: CI\n# one\n# two\non: push\n")

	_, _, err := drive(t, runFileFix, "", path)

	require.NoError(t, err)
	assert.Contains(t, body(t, path), "# one two")
}

// A directory argument reaches every file under it.
func TestADirectoryIsWalked(t *testing.T) {
	path := put(t, ".github/workflows/ci.yml", "name: CI\n# one\n# two\non: push\n")
	root := filepath.Dir(filepath.Dir(filepath.Dir(path)))

	_, _, err := drive(t, runFileFix, "", root)

	require.NoError(t, err)
	assert.Contains(t, body(t, path), "# one two")
}

// Text that is not on disk yet is what a language server holds.
func TestStdinIsJudgedAgainstTheNamedPath(t *testing.T) {
	t.Serial()
	fileJSON = true
	filePath = ".github/workflows/ci.yml"

	out, _, err := drive(t, runFileCheck, "name: CI\n# one\n# two\non: push\n")

	require.ErrorIs(t, err, errFindings)
	assert.Contains(t, out, "yaml/comment-block")
}

// Without a path nothing decides which rules read the text, and guessing would
// report a workflow's comments as prose.
func TestStdinWithNoPathIsAnError(t *testing.T) {
	_, _, err := drive(t, runFileCheck, "some text\n")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--path is required")
}

func TestOnlyNarrowsTheReport(t *testing.T) {
	t.Serial()
	path := put(t, ".github/workflows/ci.yml", "name: CI\n# one\n# two\non: push\n")
	fileOnly = []string{"yaml/all-builds-job"}

	_, _, err := drive(t, runFileCheck, "", path)

	require.NoError(t, err, "the one rule named finds nothing here")
}

// A typo that quietly selects nothing reads exactly like a clean file.
func TestATypoInOnlyIsAnError(t *testing.T) {
	t.Serial()
	path := put(t, "doc.md", "fine\n")
	fileOnly = []string{"yaml/nosuch"}

	_, _, err := drive(t, runFileCheck, "", path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown rule")
}
