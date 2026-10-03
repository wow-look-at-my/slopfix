package slopfix_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestFixKeepsMarkdownOutsideTheRoot(t *testing.T) {
	root := gitRoot(t, map[string]string{"README.md": "Front page.\n", "docs/x.md": "Notes.\n"})
	out := slopfix.FixTree(root)
	assert.FileExists(t, filepath.Join(root, "docs", "x.md"))
	for _, f := range out.Findings {
		assert.False(t, slopfix.RepoIDs.Contains(f.ID), "no repo rule judges docs/x.md: %s", f.ID)
	}
}

func TestCheckReportsAnOverBudgetFileAndChangesNothing(t *testing.T) {
	agents := "## Topic\n\n" + strings.Repeat("word ", slopfix.CharBudget/5+10) + "\n"
	root := gitRoot(t, map[string]string{"AGENTS.md": agents})
	assert.Contains(t, ids(slopfix.CheckTree(root).Findings), slopfix.IDBudget)
	got, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	require.NoError(t, err)
	assert.Equal(t, agents, string(got))
}

func TestFixSplitsAnOverBudgetFileIntoDocs(t *testing.T) {
	agents := "# Guide\n\n## Topic\n\n" + strings.Repeat("word ", slopfix.CharBudget/5+10) + "\n"
	root := gitRoot(t, map[string]string{"AGENTS.md": agents})
	out := slopfix.FixTree(root)
	assert.NotContains(t, ids(out.Findings), slopfix.IDBudget)
	assert.FileExists(t, filepath.Join(root, "docs", "topic.md"))
}

// Every request loads a nested CLAUDE.md and an imported snippet whole too, so
// the budget reads them. Another long markdown file is the control.
func TestTheBudgetReadsEveryInstructionFile(t *testing.T) {
	long := "# Guide\n\n## Topic\n\n" + strings.Repeat("word ", slopfix.CharBudget/5+10) + "\n"
	root := gitRoot(t, map[string]string{
		"pkg/CLAUDE.md":             long,
		"claude_snippets/a-rule.md": long,
		"pkg/notes.md":              long,
	})
	var over []string
	for _, f := range slopfix.CheckTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleRepo}}).Findings {
		if f.ID == slopfix.IDBudget {
			over = append(over, filepath.ToSlash(f.Path))
		}
	}
	assert.ElementsMatch(t, []string{"pkg/CLAUDE.md", "claude_snippets/a-rule.md"}, over)

	out := slopfix.FixTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleRepo}})
	assert.NotContains(t, ids(out.Findings), slopfix.IDBudget)
	assert.FileExists(t, filepath.Join(root, "pkg", "docs", "topic.md"))
	assert.FileExists(t, filepath.Join(root, "claude_snippets", "docs", "topic.md"))
}

// A file with no heading loses its tail to a file of its own, and keeps a link.
func TestFixMovesTheTailOfAFileWithNoHeading(t *testing.T) {
	root := gitRoot(t, map[string]string{"AGENTS.md": strings.Repeat("word ", slopfix.CharBudget/5+10) + "\n"})
	assert.NotContains(t, ids(slopfix.FixTree(root).Findings), slopfix.IDBudget)
	got, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	require.NoError(t, err)
	assert.LessOrEqual(t, len([]rune(string(got))), slopfix.SplitTarget)
	assert.Contains(t, string(got), "[docs/agents-continued.md](docs/agents-continued.md) holds the rest of this file.")
	assert.FileExists(t, filepath.Join(root, "docs", "agents-continued.md"))
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
	root := gitRoot(t, map[string]string{"CLAUDE.md": "Run the tests.\n", "sub/notes.md": "Notes.\n"})
	out := slopfix.CheckTree(filepath.Join(root, "sub"))
	assert.NotContains(t, ids(out.Findings), slopfix.IDAgentsFile)
}

func TestNamingAnotherRuleLeavesTheRepoRulesOut(t *testing.T) {
	root := gitRoot(t, map[string]string{"CLAUDE.md": "Run the tests.\n"})
	out := slopfix.CheckTreeWith(root, slopfix.Request{Rules: []slopfix.Rule{slopfix.RuleSTE}})
	assert.NotContains(t, ids(out.Findings), slopfix.IDAgentsFile)
}
