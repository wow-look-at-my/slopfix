package autoallow

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// homed points the rule at a scratch home, so a test never depends on the
// machine running it.
func homed(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// verdict drives the shipped table on a payload and reads the decision back.
func verdict(t *testing.T, event, tool, cwd string, input map[string]any) (string, string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": event,
		"tool_name":       tool,
		"cwd":             cwd,
		"tool_input":      input,
	})
	require.NoError(t, err)

	res := Run(bytes.NewReader(payload))
	if res.Stdout == "" {
		return "", ""
	}
	if event == eventPreToolUse {
		var resp PreToolUseResponse
		require.NoError(t, json.Unmarshal([]byte(res.Stdout), &resp))
		return resp.HookSpecificOutput.PermissionDecision, resp.HookSpecificOutput.PermissionDecisionReason
	}
	var resp PermissionResponse
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &resp))
	return resp.HookSpecificOutput.Decision.Behavior, resp.HookSpecificOutput.Decision.Message
}

func pluginTree(home string, rest ...string) string {
	return filepath.Join(append([]string{home, ".claude", "plugins"}, rest...)...)
}

// The read tools are approved wholesale, so a read of the tree is the case the
// ordering in Run exists for.
func TestAReadOfTheInstalledPluginTreeIsRefused(t *testing.T) {
	home := homed(t)
	path := pluginTree(home, "cache", "mp", "slopfix", "CLAUDE.md")

	for _, event := range []string{eventPreToolUse, eventPermissionRequest} {
		got, why := verdict(t, event, "Read", "/repo", map[string]any{"file_path": path})

		assert.Equal(t, "deny", got, event)
		assert.Contains(t, why, "plugin source directory", event)
	}
}

func TestAWriteInsideTheInstalledPluginTreeIsRefused(t *testing.T) {
	home := homed(t)
	path := pluginTree(home, "cache", "mp", "slopfix", "manifest.json")

	got, _ := verdict(t, eventPreToolUse, "Write", "/repo", map[string]any{
		"file_path": path,
		"content":   "x",
	})

	assert.Equal(t, "deny", got)
}

func TestAnEditInsideTheInstalledPluginTreeIsRefused(t *testing.T) {
	home := homed(t)
	path := pluginTree(home, "cache", "mp", "grep", "server.go")

	got, _ := verdict(t, eventPreToolUse, "Edit", "/repo", map[string]any{
		"file_path":  path,
		"new_string": "x",
	})

	assert.Equal(t, "deny", got)
}

// The tree is refused whole. Its branches hold the same installed copies, and
// naming a branch would leave every other branch open.
func TestEveryBranchOfTheTreeIsRefused(t *testing.T) {
	home := homed(t)
	for _, branch := range []string{"cache", "marketplaces", "repos", "config.json"} {
		got, _ := verdict(t, eventPreToolUse, "Read", "/repo", map[string]any{
			"file_path": pluginTree(home, branch),
		})

		assert.Equal(t, "deny", got, branch)
	}
}

func TestTheTreeItselfIsRefused(t *testing.T) {
	home := homed(t)

	got, _ := verdict(t, eventPreToolUse, "Read", "/repo", map[string]any{
		"file_path": pluginTree(home),
	})

	assert.Equal(t, "deny", got)
}

// A search names its directory on a different key, so every key that carries a
// location is read rather than the write tools' own.
func TestASearchScopedToTheTreeIsRefused(t *testing.T) {
	home := homed(t)

	got, _ := verdict(t, eventPreToolUse, "Grep", "/repo", map[string]any{
		"pattern": "slopfix",
		"path":    pluginTree(home, "cache"),
	})

	assert.Equal(t, "deny", got)
}

func TestAGlobPatternReachingTheTreeIsRefused(t *testing.T) {
	home := homed(t)

	got, _ := verdict(t, eventPreToolUse, "Glob", "/repo", map[string]any{
		"pattern": pluginTree(home, "**", "*.json"),
	})

	assert.Equal(t, "deny", got)
}

// A server tool is refused too, which is the route a plugin's own search would
// otherwise take around this.
func TestAServerSearchScopedToTheTreeIsRefused(t *testing.T) {
	home := homed(t)

	got, _ := verdict(t, eventPreToolUse, "mcp__plugin_grep_grep__Grep", "/repo", map[string]any{
		"pattern": "x",
		"path":    pluginTree(home, "cache"),
	})

	assert.Equal(t, "deny", got)
}

// The hook sees a command before the shell has resolved anything, so each
// spelling that reaches the tree is matched as written.
func TestEverySpellingOfTheTreeInACommandIsRefused(t *testing.T) {
	homed(t)
	for _, command := range []string{
		"ls -la ~/.claude/plugins/cache",
		"ls $HOME/.claude/plugins",
		"ls ${HOME}/.claude/plugins/repos",
		"find ~/.claude/plugins -name '*.go'",
		"git status && wc -c ~/.claude/plugins/cache/x",
	} {
		got, _ := verdict(t, eventPreToolUse, "Bash", "/repo", map[string]any{"command": command})

		assert.Equal(t, "deny", got, command)
	}
}

// A relative path is resolved the way the tool itself would resolve it.
func TestARelativePathThatLandsInTheTreeIsRefused(t *testing.T) {
	home := homed(t)

	got, _ := verdict(t, eventPreToolUse, "Read", pluginTree(home, "cache"), map[string]any{
		"file_path": "mp/slopfix/CLAUDE.md",
	})

	assert.Equal(t, "deny", got)
}

// The control that proves the refusal is earned rather than blanket.
func TestWorkOutsideTheTreeIsNotRefused(t *testing.T) {
	home := homed(t)
	for name, in := range map[string]struct {
		tool  string
		input map[string]any
	}{
		"a source file":     {"Read", map[string]any{"file_path": "/repo/plugins/slopfix/CLAUDE.md"}},
		"a listing":         {"Bash", map[string]any{"command": "ls -la /repo/plugins"}},
		"a search":          {"Grep", map[string]any{"pattern": "slopfix", "path": "/repo"}},
		"the settings file": {"Read", map[string]any{"file_path": filepath.Join(home, ".claude", "settings.json")}},
		"a near neighbour":  {"Read", map[string]any{"file_path": filepath.Join(home, ".claude", "plugins-notes.md")}},
	} {
		got, _ := verdict(t, eventPreToolUse, in.tool, "/repo", in.input)

		assert.NotEqual(t, "deny", got, name)
	}
}

// A document that NAMES the tree is not a call that reaches it. This is why the
// location keys are read rather than every string on the payload.
func TestADocumentThatMerelyNamesTheTreeIsNotRefused(t *testing.T) {
	home := homed(t)

	got, _ := verdict(t, eventPreToolUse, "Write", "/repo", map[string]any{
		"file_path": "/repo/CLAUDE.md",
		"content":   "Never read " + pluginTree(home, "cache") + " directly.",
	})

	assert.NotEqual(t, "deny", got)
}

// The setting is what carries the rule, so the shipped table has to name the
// tree. A table that lost the entry would pass every case above by accident.
func TestTheShippedTableNamesTheInstalledPluginTree(t *testing.T) {
	table, err := loadXMLRules(rulesXML)
	require.NoError(t, err)

	require.NotEmpty(t, table.DenyPaths)
	found := false
	for _, rule := range table.DenyPaths {
		if rule.Prefix == "~/.claude/plugins" {
			found = true
			assert.Contains(t, rule.Message, "plugin source directory")
		}
	}
	assert.True(t, found, "the table names no plugin tree")
}
