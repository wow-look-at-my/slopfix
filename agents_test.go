package slopfix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readFile(t *testing.T, root, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, name))
	require.NoError(t, err)
	return string(content)
}

func TestMigrateRenamesClaudeToAgents(t *testing.T) {
	root := repo(t, map[string]string{"CLAUDE.md": "# Index\n\n- build with go-toolchain\n"})
	got, err := MigrateAgents(root, false)
	require.NoError(t, err)
	assert.Equal(t, Migration{Renamed: true}, got)
	assert.Equal(t, "# Index\n\n- build with go-toolchain\n", readFile(t, root, "AGENTS.md"))
	assert.Equal(t, ClaudeStub, readFile(t, root, "CLAUDE.md"), "Claude Code reads CLAUDE.md, so it must import AGENTS.md")
}

func TestMigrateAppendsClaudeToExistingAgents(t *testing.T) {
	root := repo(t, map[string]string{
		"AGENTS.md": "# Agents\n\nshared rules\n",
		"CLAUDE.md": "claude-only rules\n",
	})
	got, err := MigrateAgents(root, false)
	require.NoError(t, err)
	assert.Equal(t, Migration{Merged: true}, got)
	assert.Equal(t, "# Agents\n\nshared rules\n\nclaude-only rules\n", readFile(t, root, "AGENTS.md"))
	assert.Equal(t, ClaudeStub, readFile(t, root, "CLAUDE.md"))
}

func TestMigrateKeepsTheRestOfAClaudeFileBesideTheImport(t *testing.T) {
	root := repo(t, map[string]string{
		"AGENTS.md": "shared\n",
		"CLAUDE.md": "@AGENTS.md\n\nclaude-only\n",
	})
	_, err := MigrateAgents(root, false)
	require.NoError(t, err)
	assert.Equal(t, "shared\n\nclaude-only\n", readFile(t, root, "AGENTS.md"), "the import line must not be copied into the file it imports")
	assert.Equal(t, ClaudeStub, readFile(t, root, "CLAUDE.md"))
}

func TestMigrateDoesNotDuplicateContentAlreadyInAgents(t *testing.T) {
	root := repo(t, map[string]string{
		"AGENTS.md": "shared\n\nclaude-only\n",
		"CLAUDE.md": "claude-only\n",
	})
	_, err := MigrateAgents(root, false)
	require.NoError(t, err)
	assert.Equal(t, "shared\n\nclaude-only\n", readFile(t, root, "AGENTS.md"))
	assert.Equal(t, ClaudeStub, readFile(t, root, "CLAUDE.md"))
}

func TestMigrateIsIdempotent(t *testing.T) {
	root := repo(t, map[string]string{"CLAUDE.md": "rules\n"})
	_, err := MigrateAgents(root, false)
	require.NoError(t, err)
	again, err := MigrateAgents(root, false)
	require.NoError(t, err)
	assert.False(t, again.Changed(), "a CLAUDE.md that only imports AGENTS.md is already migrated")
	assert.Equal(t, "rules\n", readFile(t, root, "AGENTS.md"))
}

func TestMigrateLeavesASymlinkAlone(t *testing.T) {
	root := repo(t, map[string]string{"AGENTS.md": "rules\n"})
	require.NoError(t, os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")))
	got, err := MigrateAgents(root, false)
	require.NoError(t, err)
	assert.False(t, got.Changed())
	target, err := os.Readlink(filepath.Join(root, "CLAUDE.md"))
	require.NoError(t, err)
	assert.Equal(t, "AGENTS.md", target)
}

func TestMigrateWithNoClaudeFileDoesNothing(t *testing.T) {
	root := repo(t, map[string]string{"AGENTS.md": "rules\n"})
	got, err := MigrateAgents(root, false)
	require.NoError(t, err)
	assert.False(t, got.Changed())
	_, err = os.Stat(filepath.Join(root, "CLAUDE.md"))
	assert.True(t, os.IsNotExist(err), "the migration must not invent a CLAUDE.md")
}

func TestMigrateDryRunWritesNothing(t *testing.T) {
	root := repo(t, map[string]string{"CLAUDE.md": "rules\n"})
	got, err := MigrateAgents(root, true)
	require.NoError(t, err)
	assert.True(t, got.Renamed)
	assert.Equal(t, "rules\n", readFile(t, root, "CLAUDE.md"))
	_, err = os.Stat(filepath.Join(root, "AGENTS.md"))
	assert.True(t, os.IsNotExist(err))
}

func TestPurgeMigratesAndKeepsTheAgentsFile(t *testing.T) {
	root := repo(t, map[string]string{"CLAUDE.md": "rules\n", "docs/x.md": "gone"})
	result, err := Purge(root, false)
	require.NoError(t, err)
	assert.True(t, result.Agents.Renamed)
	assert.Equal(t, []string{filepath.FromSlash("docs/x.md")}, result.Deleted)
	assert.Equal(t, "rules\n", readFile(t, root, "AGENTS.md"))
}
