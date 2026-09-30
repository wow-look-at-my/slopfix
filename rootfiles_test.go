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

func TestSurveyRootLeavesEveryOtherMarkdownFile(t *testing.T) {
	root := repo(t, map[string]string{
		"README.md":           "front page",
		"docs/file-format.md": strings.Repeat("x", CharBudget+1),
		"sub/README.md":       "notes",
	})
	result, err := SurveyRoot(root, false)
	require.NoError(t, err)
	assert.Empty(t, result.OverBudget, "the budget covers only the root files")
	for _, name := range []string{"README.md", "docs/file-format.md", "sub/README.md"} {
		assert.FileExists(t, filepath.Join(root, filepath.FromSlash(name)))
	}
}

func TestSurveyRootReportsARootFileOverBudget(t *testing.T) {
	root := repo(t, map[string]string{
		"README.md": "ok",
		"CLAUDE.md": strings.Repeat("x", CharBudget+1),
	})
	result, err := SurveyRoot(root, false)
	require.NoError(t, err)
	require.Contains(t, result.OverBudget, "AGENTS.md")
	assert.Equal(t, CharBudget+2, result.OverBudget["AGENTS.md"], "the move adds a final newline")
	assert.Contains(t, BudgetError(result.OverBudget), "over the 40000 budget")
}

func TestSurveyRootCountsCharactersNotBytes(t *testing.T) {
	// An em dash is a single character spelled in several bytes. A byte count
	root := repo(t, map[string]string{"README.md": strings.Repeat("—", CharBudget)})
	result, err := SurveyRoot(root, false)
	require.NoError(t, err)
	assert.Empty(t, result.OverBudget)
}
