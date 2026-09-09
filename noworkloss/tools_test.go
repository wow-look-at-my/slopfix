package noworkloss

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The routes that are not Bash at all. Each keeps the same pair as the shell
// cases: the call that must be refused, and the neighbouring call that must
// still work, so a rule that denied the whole tool would fail the control.

func TestTheEditToolsThemselvesAreLeftAlone(t *testing.T) {
	root := newTree(t)
	existing := filepath.Join(root, "src.txt")
	for _, tool := range []string{"Edit", "MultiEdit", "NotebookEdit"} {
		assert.Empty(t, askTool(t, tool, root, map[string]any{"file_path": existing}),
			"%s is the sanctioned route and must never be denied for using it", tool)
	}
	assert.Empty(t, askTool(t, "Write", root, map[string]any{"file_path": filepath.Join(root, "new.txt")}),
		"Write is how a new file is created")
}

// Write authors a whole file, so aiming it at a path that already exists replaces
// content nobody reviewed the loss of.
func TestWriteOverAnExistingFileIsRefused(t *testing.T) {
	root := newTree(t)
	reason := askTool(t, "Write", root, map[string]any{"file_path": filepath.Join(root, "src.txt")})
	require.NotEmpty(t, reason)
	assert.Contains(t, reason, "already exists")
	assert.Contains(t, reason, "Use Edit")

	// A path outside the tree is no different: this rule is about the tool's
	out := outsideTree(t)
	assert.NotEmpty(t, askTool(t, "Write", root, map[string]any{"file_path": filepath.Join(out, "src.txt")}))
	assert.Empty(t, askTool(t, "Write", root, map[string]any{"file_path": filepath.Join(out, "fresh.txt")}))
}

// The refusal above names a path, so the path gets emptied and the same Write
// goes again. Each verb that empties it leaves the file in git and off the
// disk, and the control is the file git never held.
func TestWriteOverAPathEmptiedToGetPastTheRefusalIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		empty func(t *testing.T, dir string)
	}{
		{"renamed away", func(t *testing.T, dir string) { git(t, dir, "mv", "tracked.go", "tracked.go.old") }},
		{"deleted", func(t *testing.T, dir string) {
			require.NoError(t, os.Remove(filepath.Join(dir, "tracked.go")))
		}},
		{"git rm", func(t *testing.T, dir string) { git(t, dir, "rm", "-q", "tracked.go") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := newRepo(t)
			path := filepath.Join(dir, "tracked.go")
			require.NotEmpty(t, askTool(t, "Write", dir, map[string]any{"file_path": path}),
				"the file is still on disk here")

			tc.empty(t, dir)
			reason := askTool(t, "Write", dir, map[string]any{"file_path": path})
			require.NotEmpty(t, reason, "%s must not open a route Write was refused", tc.name)
			assert.Contains(t, reason, "git still holds tracked.go")
			assert.Contains(t, reason, "run: git -C "+dir+" restore -- tracked.go")

			// The control: a path git never held is an ordinary new file.
			assert.Empty(t, askTool(t, "Write", dir, map[string]any{"file_path": filepath.Join(dir, "fresh.go")}))
		})
	}
}

// Committing the removal is the way out. The content is then in history, where
// a reader finds it, and the path is free.
func TestWriteIsAllowedOnceTheRemovalIsCommitted(t *testing.T) {
	dir := newRepo(t)
	git(t, dir, "rm", "-q", "tracked.go")
	git(t, dir, "commit", "-qm", "drop it")
	assert.Empty(t, askTool(t, "Write", dir, map[string]any{"file_path": filepath.Join(dir, "tracked.go")}))
}

func TestEditingTheLiveSettingsIsRefused(t *testing.T) {
	root := newTree(t)
	home := t.TempDir()
	live := filepath.Join(home, ".claude", "settings.json")
	writeFile(t, live, "{}")

	reason := askTool(t, "Edit", root, map[string]any{"file_path": live})
	require.NotEmpty(t, reason, "a session must not edit the settings that gate it")
	assert.Contains(t, reason, "live Claude Code settings")

	// The control: this is about the live settings, not about every file called
	assert.Empty(t, askTool(t, "Write", root, map[string]any{"file_path": filepath.Join(root, "settings.json")}))
	assert.Empty(t, askTool(t, "Write", root, map[string]any{
		"file_path": filepath.Join(root, "plugins", "x", ".claude-plugin", "plugin.json"),
	}))
}

func TestAConfigSkillCannotReGrantWhatIsDenied(t *testing.T) {
	root := newTree(t)
	for _, skill := range []string{"update-config", "fewer-permission-prompts"} {
		reason := askTool(t, "Skill", root, map[string]any{"skill": skill})
		require.NotEmpty(t, reason, "%s rewrites settings.json as its whole purpose", skill)
		assert.Contains(t, reason, "settings")
	}
	for _, skill := range []string{"dataviz", "docs:dockerfile", "code-review"} {
		assert.Empty(t, askTool(t, "Skill", root, map[string]any{"skill": skill}),
			"%s has nothing to do with permissions", skill)
	}
}

// Delegating is fine. Handing the child something the parent does not have is
// the single-call bypass of everything else in this plugin.
func TestASubagentCannotBeHandedAWiderGrant(t *testing.T) {
	root := newTree(t)
	widened := []map[string]any{
		{"prompt": "fix it", "permissionMode": "bypassPermissions"},
		{"prompt": "fix it", "permissionMode": "acceptEdits"},
		{"prompt": "fix it", "permission_mode": "dontAsk"},
		{"prompt": "fix it", "tools": []string{"Read", "Bash"}},
		{"prompt": "fix it", "allowedTools": []string{"Bash"}},
		{"prompt": "fix it", "extra_allowed_tools": []string{"Bash"}},
		{"prompt": "fix it", "allowedTools": "Read,Bash"},
	}
	for _, input := range widened {
		for _, tool := range []string{"Agent", "Task", "mcp__Claude_Code_Remote__create_session"} {
			reason := askTool(t, tool, root, input)
			require.NotEmpty(t, reason, "%s with %v must be refused", tool, input)
			assert.Contains(t, reason, "child")
		}
	}

	ordinary := []map[string]any{
		{"prompt": "research the parser", "subagent_type": "Explore"},
		{"prompt": "review this", "model": "sonnet"},
		{"prompt": "look around", "tools": []string{"Read", "Grep"}},
		{"prompt": "plan it", "permissionMode": "plan"},
	}
	for _, input := range ordinary {
		assert.Empty(t, askTool(t, "Agent", root, input),
			"ordinary delegation must not be denied: %v", input)
	}
}

func TestTheGitHubContentAPITools(t *testing.T) {
	root := newTree(t)
	denied := []string{
		"mcp__github__create_or_update_file",
		"mcp__github__push_files",
		"mcp__github__delete_file",
		"mcp__GitHub_API_MCP__create_or_update_file",
		"mcp__GitHub_API_MCP__push_files",
	}
	for _, tool := range denied {
		reason := askTool(t, tool, root, map[string]any{"path": "README.md", "content": "x"})
		require.NotEmpty(t, reason, "%s commits content no edit tool ever saw", tool)
		assert.Contains(t, reason, "never exists as a file")
	}

	allowed := []string{
		"mcp__github__get_file_contents",
		"mcp__github__list_commits",
		"mcp__github__pull_request_read",
		"mcp__github__update_pull_request",
		"mcp__plugin_grep_grep__Grep",
	}
	for _, tool := range allowed {
		assert.Empty(t, askTool(t, tool, root, map[string]any{"path": "README.md"}),
			"%s reads, or edits metadata rather than file content", tool)
	}
}

func TestNothingIsWrittenForAnAllowedCall(t *testing.T) {
	root := newTree(t)
	out := captureStdout(t, func() {
		if r := ask(t, root, "git status"); r != "" {
			emitDeny(r)
		}
	})
	assert.Empty(t, out, "an allowed call must leave the normal permission flow untouched")
}

func TestOtherEventsAndToolsAreIgnored(t *testing.T) {
	root := newTree(t)
	for _, event := range []string{"PostToolUse", "Stop", "SessionStart", ""} {
		assert.Empty(t, decideWithEvent(t, event, "Bash", root, "sed -i s/a/b/ src.txt"),
			"%s is not this hook's event", event)
	}
	assert.Empty(t, ask(t, root, ""), "an empty command decides nothing")
	unparseable, notices := decide([]byte("not json"))
	assert.Empty(t, unparseable)
	assert.Empty(t, notices, "an unparseable payload preserves nothing, so it announces nothing")
}
