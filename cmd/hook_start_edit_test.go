package cmd

import _ "embed"

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/forkscope"
)

// widgetRs opens on code and carries a doc comment further down, so the header rule has prose to copy from.
//
//go:embed testdata/widget-rs.rs
var widgetRs string

// askIn is ask with the fork resolver the hook reads.
func askIn(t *testing.T, forks forkscope.Resolver, payload map[string]any, only ...string) answer {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	rules, ids, err := selectedRules(only)
	require.NoError(t, err)

	body := judge(data, rules, ids, forks)
	if body == "" {
		return answer{}
	}
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &raw))
	out, _ := raw["hookSpecificOutput"].(map[string]any)
	return answer{raw: raw, out: out, body: body}
}

// landed answers the file the tool writes: the payload as sent, or as the hook rewrote it.
func landed(t *testing.T, before string, payload map[string]any, got answer) string {
	t.Helper()
	require.NotContains(t, got.body, "permissionDecision", got.body)
	if got.out == nil || got.out["updatedInput"] == nil {
		input, _ := payload["tool_input"].(map[string]any)
		old, _ := input["old_string"].(string)
		text, _ := input["new_string"].(string)
		return strings.Replace(before, old, text, 1)
	}
	return applied(t, before, got)
}

// An edit that opens at the first byte of a file keeps its first line. The
// header rule writes above that line, and the repair of the new text must not
// read the header as part of it.
func TestAnEditAtTheFirstByteKeepsItsFirstLine(t *testing.T) {
	path := onDisk(t, "widget.rs", widgetRs)
	payload := editOf(path, "use std::fmt;", "use std::fmt::Write;")
	got := ask(t, payload, "comments")

	want := strings.Replace(widgetRs, "use std::fmt;", "use std::fmt::Write;", 1)
	assert.Equal(t, want, landed(t, widgetRs, payload, got))
}

// The same edit in a fork file whose header line the fork did not write.
func TestAnEditAtTheFirstByteOfAForkFileKeepsItsFirstLine(t *testing.T) {
	fx := newForkRepoWith(t, aFork, map[string]string{"widget.rs": widgetRs})
	path := filepath.Join(fx.dir, "widget.rs")
	payload := editOf(path, "use std::fmt;", "use std::fmt::Write;")
	got := askIn(t, fx.forks, payload, "comments")

	want := strings.Replace(widgetRs, "use std::fmt;", "use std::fmt::Write;", 1)
	assert.Equal(t, want, landed(t, widgetRs, payload, got))
}

// An edit a little below the first byte of a fork file lands as it was sent.
func TestAnEditNearTheStartOfAForkFileLandsAsSent(t *testing.T) {
	fx := newForkRepoWith(t, aFork, map[string]string{"widget.rs": widgetRs})
	path := filepath.Join(fx.dir, "widget.rs")
	payload := editOf(path, "use std::io;", "use std::io::Read;")
	got := askIn(t, fx.forks, payload, "comments")

	want := strings.Replace(widgetRs, "use std::io;", "use std::io::Read;", 1)
	assert.Equal(t, want, landed(t, widgetRs, payload, got))
}

// An edit to a comment lands with the words the edit wrote.
func TestAnEditToACommentKeepsTheEditedWords(t *testing.T) {
	fx := newForkRepoWith(t, aFork, map[string]string{"widget.rs": widgetRs})
	path := filepath.Join(fx.dir, "widget.rs")
	payload := editOf(path, "/// Counts the widgets in stock.", "/// Counts the widgets on the shelf.")
	got := askIn(t, fx.forks, payload, "comments")

	want := strings.Replace(widgetRs, "in stock", "on the shelf", 1)
	assert.Equal(t, want, landed(t, widgetRs, payload, got))
}

// A MultiEdit of one replacement is repaired in that replacement.
func TestAMultiEditOfOneEditIsRepairedInPlace(t *testing.T) {
	path := onDisk(t, "a.go", "package p\n\nfunc x() {}\n")
	payload := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "MultiEdit",
		"tool_input": map[string]any{
			"file_path": path,
			"edits": []any{
				map[string]any{"old_string": "func x() {}", "new_string": "// This used to read the flag.\nfunc x() {}"},
			},
		},
	}
	got := ask(t, payload, "tombstones")

	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated, got.body)
	assert.NotContains(t, updated, "new_string")
	edits, _ := updated["edits"].([]any)
	require.Len(t, edits, 1)
	first, _ := edits[0].(map[string]any)
	assert.Equal(t, "func x() {}", first["new_string"])
}
