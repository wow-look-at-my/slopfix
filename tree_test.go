package slopfix_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
)

// tree writes each named file under a single root and answers it.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.25\n"), 0o600))
	for name, content := range files {
		path := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	return root
}

func read(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

// The comment sweep reached a single rule family. A build calling this repairs
// a workflow and a document in the same pass, which is the whole point.
func TestFixTreeRepairsEveryRuleFamily(t *testing.T) {
	root := tree(t, map[string]string{
		".github/workflows/ci.yml": "name: CI\n# one\n# two\non: push\n",
		"notes.md":                 "It has three plugins.\n",
	})

	out := slopfix.FixTree(root)

	assert.Contains(t, read(t, filepath.Join(root, ".github/workflows/ci.yml")), "# one two")
	assert.Contains(t, read(t, filepath.Join(root, "notes.md")), "It has plugins.")
	assert.Len(t, out.Repaired, 2)
}

// A file no rule reads is not opened, so a tree of them costs no repair.
func TestFixTreeLeavesAFileNoRuleReads(t *testing.T) {
	root := tree(t, map[string]string{"data.bin": "\x00\x01\x02"})

	out := slopfix.FixTree(root)

	assert.Empty(t, out.Repaired)
	assert.Equal(t, "\x00\x01\x02", read(t, filepath.Join(root, "data.bin")))
}

// A clean tree is rewritten nowhere, which is what makes the sweep safe to run
// on every build.
func TestFixTreeWritesNothingWhenThereIsNothingToRepair(t *testing.T) {
	root := tree(t, map[string]string{".github/workflows/ci.yml": "name: CI\n# one\non: push\n"})
	path := filepath.Join(root, ".github/workflows/ci.yml")
	before, err := os.Stat(path)
	require.NoError(t, err)

	out := slopfix.FixTree(root)

	assert.Empty(t, out.Repaired)
	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime(), "a clean file was rewritten")
}

func TestFixTreeQuotesWhatItRemoved(t *testing.T) {
	root := tree(t, map[string]string{"notes.md": "It has three plugins.\n"})

	out := slopfix.FixTree(root)

	require.NotEmpty(t, out.Repaired)
	assert.Equal(t, root+"/notes.md", out.Repaired[0])
}

func TestReadsAnswersTheFilesEveryRuleCovers(t *testing.T) {
	assert.True(t, slopfix.Reads("notes.md"))
	assert.True(t, slopfix.Reads(".github/workflows/ci.yml"))
	assert.True(t, slopfix.Reads("main.go"))
	assert.False(t, slopfix.Reads("data.bin"))
}
