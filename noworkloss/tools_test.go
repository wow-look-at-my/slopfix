package noworkloss

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The routes that are not Bash at all. Each keeps the same pair as the shell
// cases. The call that must be refused, and the neighbouring call that must
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

// Write authors a whole file.
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
// disk. The hook puts the file back rather than only refusing, so Edit has
// something to work on. The control is the file git never held.
func TestWriteOverAPathEmptiedToGetPastTheRefusalRestoresIt(t *testing.T) {
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
			require.NoFileExists(t, path, "%s empties the path", tc.name)

			reason := askTool(t, "Write", dir, map[string]any{"file_path": path})
			require.NotEmpty(t, reason, "%s must not open a route Write was refused", tc.name)
			assert.Contains(t, reason, "put the file back")
			assert.Contains(t, reason, "Use the Edit tool")

			// The repair is the point: the file is on disk with its content.
			restored, err := os.ReadFile(path)
			require.NoError(t, err, "the hook must put the file back, not only refuse")
			assert.Equal(t, "package a\n", string(restored))

			// The control: a path git never held is an ordinary new file.
			assert.Empty(t, askTool(t, "Write", dir, map[string]any{"file_path": filepath.Join(dir, "fresh.go")}))
		})
	}
}

// fakeRecycler puts a `recycler` on PATH whose bin holds the given items. A
// restore writes "kept" to the item's path and logs the ID in restored-ids. It
// fails when the item ID is "broken". The result is the fake's directory.
func fakeRecycler(t *testing.T, items []binItem) string {
	bin := t.TempDir()
	listing, err := json.Marshal(items)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(bin, "items.json"), listing, 0o644))
	script := `#!/bin/sh
set -eu
dir=$(dirname "$0")
case "$1" in
list) cat "$dir/items.json" ;;
restore)
	[ "$2" = broken ] && { echo "recycler: restore failed" >&2; exit 1; }
	echo "$2" >> "$dir/restored-ids"
	printf 'kept\n' > "$(jq -r --arg id "$2" '.[] | select(.id == $id) | .original_path' "$dir/items.json")"
	;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, "recycler"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return bin
}

func binnedAt(id, path, deleted string) binItem {
	at, err := time.Parse(time.RFC3339, deleted)
	if err != nil {
		panic(err)
	}
	return binItem{ID: id, OriginalPath: path, DeletedAt: at}
}

// A recycled file comes back before the Write is refused. Several Writes go
// out in one batch, and only a restore the hook makes itself reaches them all.
func TestWriteOverARecycledFileRestoresIt(t *testing.T) {
	out := outsideTree(t)
	path := filepath.Join(out, "bench.js")
	bin := fakeRecycler(t, []binItem{
		binnedAt("old", path, "2026-01-01T00:00:00Z"),
		binnedAt("new", path, "2026-02-01T00:00:00Z"),
	})

	reason := askTool(t, "Write", out, map[string]any{"file_path": path})
	assert.Contains(t, reason, "put the file back")
	assert.Contains(t, reason, "Edit tool")
	restored, err := os.ReadFile(path)
	require.NoError(t, err, "the hook must put the file back, not only refuse")
	assert.Equal(t, "kept\n", string(restored))

	ids, err := os.ReadFile(filepath.Join(bin, "restored-ids"))
	require.NoError(t, err)
	assert.Equal(t, "new\n", string(ids), "the newest item at the path is the one restored")

	// The control: a path the bin never held is an ordinary new file.
	assert.Empty(t, askTool(t, "Write", out, map[string]any{"file_path": filepath.Join(out, "fresh.js")}))
}

// A restore that fails still refuses, and names the command by item ID.
func TestAFailedBinRestoreNamesTheCommand(t *testing.T) {
	out := outsideTree(t)
	path := filepath.Join(out, "bench.js")
	fakeRecycler(t, []binItem{binnedAt("broken", path, "2026-01-01T00:00:00Z")})

	reason := askTool(t, "Write", out, map[string]any{"file_path": path})
	assert.Contains(t, reason, "restore failed")
	assert.Contains(t, reason, "recycler restore broken")
	assert.NoFileExists(t, path)
}

// The restore writes the working tree and nothing else. A `git rm` staged the
// removal, and that staged state is the session's to keep or undo.
func TestTheRestoreLeavesTheIndexAlone(t *testing.T) {
	dir := newRepo(t)
	git(t, dir, "rm", "-q", "tracked.go")
	staged := gitOutput(t, dir, "diff", "--cached", "--name-status")

	require.NotEmpty(t, askTool(t, "Write", dir, map[string]any{"file_path": filepath.Join(dir, "tracked.go")}))
	assert.Equal(t, staged, gitOutput(t, dir, "diff", "--cached", "--name-status"),
		"the index must read exactly as it did")
}

// Committing the removal frees nothing. A deletion committed only to get past
// the refusal is the same evasion one step later. The file comes back from
// the commit before the deletion and the Write is refused with its price.
func TestACommittedDeletionDoesNotFreeThePathForWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		remove func(t *testing.T, dir string)
	}{
		{"git rm", func(t *testing.T, dir string) { git(t, dir, "rm", "-q", "tracked.go") }},
		{"trashed then add -A", func(t *testing.T, dir string) {
			require.NoError(t, os.Remove(filepath.Join(dir, "tracked.go")))
			git(t, dir, "add", "-A")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := newRepo(t)
			path := filepath.Join(dir, "tracked.go")
			tc.remove(t, dir)
			git(t, dir, "commit", "-qm", "remove old tests")
			require.NoFileExists(t, path)

			reason := askWrite(t, dir, path, "package a\n\nfunc Fresh() {}\n", transcriptFixture(t, path))
			require.NotEmpty(t, reason, "a committed deletion must not open a route Write was refused")
			assert.Contains(t, reason, "Use the Edit tool")
			assert.NotContains(t, reason, "Commit the removal")
			assert.Regexp(t, `Tokens wasted on this evasion compared to one Edit: -?\d+ \(method: `, reason)

			restored, err := os.ReadFile(path)
			require.NoError(t, err, "the hook must put the file back from the commit before the deletion")
			assert.Equal(t, "package a\n", string(restored))

			// The control: a path git never held is an ordinary new file.
			assert.Empty(t, askWrite(t, dir, filepath.Join(dir, "fresh.go"), "package a\n", transcriptFixture(t, path)))
		})
	}
}

// The counter is extra. A transcript that cannot be read leaves the refusal
// and the restore in place, and says the count is unavailable.
func TestTheRefusalStandsWithoutATranscript(t *testing.T) {
	dir := newRepo(t)
	path := filepath.Join(dir, "tracked.go")
	git(t, dir, "rm", "-q", "tracked.go")
	git(t, dir, "commit", "-qm", "drop it")

	reason := askWrite(t, dir, path, "package a\n", filepath.Join(t.TempDir(), "missing.jsonl"))
	require.NotEmpty(t, reason)
	assert.Contains(t, reason, "Use the Edit tool")
	assert.Contains(t, reason, "unavailable")
	assert.FileExists(t, path)
}

// Recent is bounded. A file deleted further back than the window, on a branch
// the default branch already holds, is created afresh like any new file.
func TestAnOldDeletionDoesNotHoldThePath(t *testing.T) {
	dir := newRepo(t)
	git(t, dir, "rm", "-q", "tracked.go")
	git(t, dir, "commit", "-qm", "drop it")
	for i := 0; i <= recentDepth; i++ {
		git(t, dir, "commit", "-q", "--allow-empty", "-m", "later")
	}
	assert.Empty(t, askWrite(t, dir, filepath.Join(dir, "tracked.go"), "package a\n", ""))
}

// The evasion is the run of calls after the last Read of the file: the trash,
// the refused Write and the commit. The earlier commit, the unrelated vet and
// the call being decided are not in it.
func TestTheEvasionIsTheRunSinceTheFileWasIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tracked.go")
	calls, err := readToolCalls(transcriptFixture(t, path))
	require.NoError(t, err)
	var ids []string
	for _, c := range evasion(calls, path, "toolu_final") {
		ids = append(ids, c.id)
	}
	assert.Equal(t, []string{"toolu_trash", "toolu_write1", "toolu_commit"}, ids)
}

func TestChangedLinesAreWhatAnEditCarries(t *testing.T) {
	removed, added := changedLines("a\nb\nc\nd\n", "a\nB\nc\nd\ne\n")
	assert.Equal(t, "b\n", removed)
	assert.Equal(t, "B\ne\n", added)
}

// askWrite is askTool for a Write, with the transcript and tool_use_id the
// counter reads.
func askWrite(t *testing.T, cwd, path, content, transcript string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_use_id":     "toolu_final",
		"cwd":             cwd,
		"transcript_path": transcript,
		"tool_input":      map[string]any{"file_path": path, "content": content},
	})
	require.NoError(t, err)
	reason, _ := decide(raw)
	return reason
}

// transcriptFixture writes testdata/evasion.jsonl with {{path}} naming path.
func transcriptFixture(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "evasion.jsonl"))
	require.NoError(t, err)
	quoted, err := json.Marshal(path)
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeFile(t, out, strings.ReplaceAll(string(body), "{{path}}", strings.Trim(string(quoted), `"`)))
	return out
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
