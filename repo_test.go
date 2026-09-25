package slopfix_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

// gitRoot builds a repository root: the repo rules judge a tree only from there.
func gitRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	return root
}

func ids(findings []slopfix.TreeFinding) []string {
	var out []string
	for _, f := range findings {
		out = append(out, f.ID)
	}
	return out
}

func TestCheckReportsAStrayMarkdownFileAndDeletesNothing(t *testing.T) {
	root := gitRoot(t, map[string]string{"README.md": "Front page.\n", "docs/x.md": "Notes.\n"})
	out := slopfix.CheckTree(root)
	assert.Contains(t, ids(out.Findings), slopfix.IDStrayMarkdown)
	assert.FileExists(t, filepath.Join(root, "docs", "x.md"))
}

func TestFixDeletesAStrayMarkdownFile(t *testing.T) {
	root := gitRoot(t, map[string]string{"README.md": "Front page.\n", "docs/x.md": "Notes.\n"})
	out := slopfix.FixTree(root)
	assert.NoFileExists(t, filepath.Join(root, "docs", "x.md"))
	assert.NotContains(t, ids(out.Findings), slopfix.IDStrayMarkdown)
	assert.FileExists(t, filepath.Join(root, "README.md"))
}

func TestFixMovesClaudeIntoAgents(t *testing.T) {
	root := gitRoot(t, map[string]string{"CLAUDE.md": "Run the tests.\n"})
	assert.Contains(t, ids(slopfix.CheckTree(root).Findings), slopfix.IDAgentsFile)

	slopfix.FixTree(root)
	claude, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, slopfix.ClaudeStub, string(claude))
	assert.FileExists(t, filepath.Join(root, "AGENTS.md"))
}

func TestAWalkBelowTheRootLeavesTheRepoRulesOut(t *testing.T) {
	root := gitRoot(t, map[string]string{"sub/notes.md": "Notes.\n"})
	out := slopfix.CheckTree(filepath.Join(root, "sub"))
	assert.NotContains(t, ids(out.Findings), slopfix.IDStrayMarkdown)
}

func TestTestdataIsNotJudged(t *testing.T) {
	root := gitRoot(t, map[string]string{"pkg/testdata/case.md": "A fixture.\n"})
	assert.NotContains(t, ids(slopfix.CheckTree(root).Findings), slopfix.IDStrayMarkdown)
}

func TestNamingAnotherRuleLeavesTheRepoRulesOut(t *testing.T) {
	root := gitRoot(t, map[string]string{"docs/x.md": "Notes.\n"})
	out := slopfix.CheckTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleSTE}})
	assert.NotContains(t, ids(out.Findings), slopfix.IDStrayMarkdown)
}
