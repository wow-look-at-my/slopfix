package bashclean

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mvdan.cc/sh/v3/syntax"
)

func payload(t *testing.T, tool, command string) string {
	t.Helper()
	in := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       tool,
		"tool_input":      map[string]any{"command": command, "description": "keep me"},
	}
	b, err := json.Marshal(in)
	require.NoError(t, err)
	return string(b)
}

func parseStmts(t *testing.T, src string) []*syntax.Stmt {
	t.Helper()
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(src), "")
	require.NoError(t, err)
	return f.Stmts
}

// Silence is the fail-open answer, and it is every failure path as well.
func TestRunFailsOpen(t *testing.T) {
	for name, in := range map[string]string{
		"invalid JSON":     "not json",
		"empty stdin":      "",
		"non-Bash tool":    payload(t, "Read", "ls 2>/dev/null"),
		"missing command":  `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{}}`,
		"other event":      `{"hook_event_name":"Stop","tool_name":"Bash","tool_input":{"command":"ls 2>/dev/null"}}`,
		"unparseable bash": payload(t, "Bash", "if true; then"),
		"nothing to do":    payload(t, "Bash", "set -o pipefail; ls"),
	} {
		res := Run(strings.NewReader(in))
		assert.Empty(t, res.Stdout, name)
		assert.Equal(t, 0, res.Code, name)
	}
}

// A rewrite carries updatedInput and nothing else: no permissionDecision, no
// systemMessage. The rest of tool_input rides along untouched.
func TestRunRewriteIsSilent(t *testing.T) {
	res := Run(strings.NewReader(payload(t, "Bash", "ls 2>/dev/null")))
	require.NotEmpty(t, res.Stdout)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out))
	assert.Equal(t, true, out["suppressOutput"])
	assert.NotContains(t, out, "systemMessage")

	hso := out["hookSpecificOutput"].(map[string]any)
	assert.Equal(t, "PreToolUse", hso["hookEventName"])
	assert.NotContains(t, hso, "permissionDecision")
	assert.NotContains(t, hso, "additionalContext")

	updated := hso["updatedInput"].(map[string]any)
	assert.Equal(t, "set -o pipefail\nls\n", updated["command"])
	assert.Equal(t, "keep me", updated["description"])
}

// A deny carries the reason and no updatedInput. The reason names the tool or
// the command that does the job instead, or the model retries the same thing.
func TestRunDenyCarriesTheAlternative(t *testing.T) {
	for command, want := range map[string]string{
		"cat <<EOF\nx\nEOF\n":        "Write/Edit",
		"perl -e 'print 1'":          "perl is banned",
		"head -60 src/usage.test.ts": "offset and limit",
		"shred secret.txt":           "recycler trash",
		"git rm f":                   "git add -A",
		"truncate -s 0 -r ref f":     "recycler trash",
		"rm --one-file-system x":     "cannot be translated",
	} {
		res := Run(strings.NewReader(payload(t, "Bash", command)))
		require.NotEmpty(t, res.Stdout, command)

		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(res.Stdout), &out))
		hso := out["hookSpecificOutput"].(map[string]any)
		assert.Equal(t, "deny", hso["permissionDecision"], command)
		assert.NotContains(t, hso, "updatedInput", command)
		assert.NotContains(t, out, "systemMessage", command)
		assert.Contains(t, hso["permissionDecisionReason"], want, command)
	}
}

// The hook is silent toward user and model, so the log file is the only trace
// a rewrite leaves.
func TestRunLogsToTheDebugChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cleanup.log")
	t.Setenv("CLEANUP_BASH_CMDS_LOG", path)

	Run(strings.NewReader(payload(t, "Bash", "rm -rf build")))
	Run(strings.NewReader(payload(t, "Bash", "perl -e 'print 1'")))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `REWRITE`)
	assert.Contains(t, string(data), `rules="rm_recycle,pipefail"`)
	assert.Contains(t, string(data), `DENY`)
	assert.Contains(t, string(data), `reason="perl"`)
}

// The transform only maps over, edits within, or prepends to the statement
// list, so a rewrite can add a statement and never drop one.
func TestRewriteNeverDropsAStatement(t *testing.T) {
	for _, in := range []string{
		"echo start\nsleep 10\nls | tail -5",
		"cd /repo\nA=1\necho \"=== files ===\"\nls -1 \"$A\" 2>&1 | tail -12\necho \"=== done ===\"\n./run a b",
	} {
		before := len(parseStmts(t, in))
		after := len(parseStmts(t, Transform(in).Command))
		assert.GreaterOrEqual(t, after, before, "statements lost rewriting %q", in)
	}
}
