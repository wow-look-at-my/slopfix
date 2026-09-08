package slopfix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repo builds a tree from a name-to-content map and returns its root.
func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return root
}

func TestPurgeKeepsTheRootPair(t *testing.T) {
	root := repo(t, map[string]string{
		"README.md":           "front page",
		"CLAUDE.md":           "agent index",
		"docs/file-format.md": "a reference nobody reads",
		"docs/nested/deep.md": "more of the same",
		"pkg/NOTES.md":        "scratch",
		"main.go":             "package main",
		"docs/schema.json":    "{}",
	})

	result, err := Purge(root, false)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		filepath.FromSlash("docs/file-format.md"),
		filepath.FromSlash("docs/nested/deep.md"),
		filepath.FromSlash("pkg/NOTES.md"),
	}, result.Deleted)

	for _, kept := range []string{"README.md", "CLAUDE.md", "main.go", "docs/schema.json"} {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(kept)))
		assert.NoError(t, err, "%s must survive", kept)
	}
	_, err = os.Stat(filepath.Join(root, "docs", "file-format.md"))
	assert.True(t, os.IsNotExist(err), "a doc under docs/ must be gone")
}

func TestPurgeDeletesAReadmeThatIsNotAtTheRoot(t *testing.T) {
	// The pair is kept by name AND position. A README in a subdirectory is
	root := repo(t, map[string]string{"README.md": "keep", "sub/README.md": "go"})
	result, err := Purge(root, false)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.FromSlash("sub/README.md")}, result.Deleted)
}

func TestPurgeDryRunDeletesNothing(t *testing.T) {
	root := repo(t, map[string]string{"docs/thing.md": "prose"})
	result, err := Purge(root, true)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.FromSlash("docs/thing.md")}, result.Deleted)
	_, err = os.Stat(filepath.Join(root, "docs", "thing.md"))
	assert.NoError(t, err, "a dry run must not delete")
}

func TestPurgeSkipsASpecRepo(t *testing.T) {
	root := repo(t, map[string]string{".slopfix-spec": "", "src/lexer.md": "the product"})
	result, err := Purge(root, false)
	require.NoError(t, err)
	assert.Empty(t, result.Deleted)
	_, err = os.Stat(filepath.Join(root, "src", "lexer.md"))
	assert.NoError(t, err)
}

func TestPurgeIgnoresVendoredTrees(t *testing.T) {
	root := repo(t, map[string]string{
		".git/hooks/README.md":       "git's own",
		"node_modules/pkg/README.md": "somebody else's",
		"vendor/dep/DESIGN.md":       "somebody else's",
		"docs/ours.md":               "ours",
	})
	result, err := Purge(root, false)
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.FromSlash("docs/ours.md")}, result.Deleted)
}

func TestPurgeReportsAKeptFileOverBudget(t *testing.T) {
	root := repo(t, map[string]string{
		"README.md": "ok",
		"CLAUDE.md": strings.Repeat("x", CharBudget+1),
	})
	result, err := Purge(root, false)
	require.NoError(t, err)
	assert.Empty(t, result.Deleted)
	require.Contains(t, result.OverBudget, "CLAUDE.md")
	assert.Equal(t, CharBudget+1, result.OverBudget["CLAUDE.md"])
	assert.Contains(t, BudgetError(result.OverBudget), "over the 40000 budget")
}

func TestPurgeCountsCharactersNotBytes(t *testing.T) {
	// An em dash is a single character spelled in several bytes. A byte count
	root := repo(t, map[string]string{"README.md": strings.Repeat("—", CharBudget)})
	result, err := Purge(root, false)
	require.NoError(t, err)
	assert.Empty(t, result.OverBudget)
}
