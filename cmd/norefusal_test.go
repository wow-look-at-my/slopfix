package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reported is a file the rules already report: the comment states a count.
const reported = `package p

// The walk has 3 phases.
func walk() {}
`

// The hook repairs a write and lets it through. Nothing it judges is grounds
// for refusing the write, so a comment reworded by hand is ordinary work.
func TestNoWriteIsEverRefused(t *testing.T) {
	path := onDisk(t, "a.go", reported)
	for _, c := range []struct {
		name, old, new string
	}{
		{
			name: "a comment reworded by hand in a reported file",
			old:  "// The walk has 3 phases.",
			new:  "// The walk runs in phases.",
		},
		{
			name: "a comment the author writes afresh",
			old:  "func walk() {}",
			new:  "// walk reaches every node.\nfunc walk() {}",
		},
		{
			name: "an edit that moves the code under the comment",
			old:  "// The walk has 3 phases.\nfunc walk() {}",
			new:  "// The walk runs in phases.\nfunc walk() error { return nil }",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			out := drive(t, editPayload(t, path, c.old, c.new))
			assert.NotContains(t, out, "permissionDecision")
			assert.NotContains(t, out, "blocked")
		})
	}
}

// editPayload builds the PreToolUse envelope for an Edit.
func editPayload(t *testing.T, path, old, new string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input":      map[string]any{"file_path": path, "old_string": old, "new_string": new},
	})
	require.NoError(t, err)
	return string(data)
}

// onDisk writes a file and answers its path.
func onDisk(t *testing.T, name, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(src), 0o600))
	return path
}

// drive runs the hook against a payload and answers what it prints.
func drive(t *testing.T, payload string) string {
	t.Helper()
	cmdMu.Lock()
	defer cmdMu.Unlock()
	c := find(t, "hook")
	var out strings.Builder
	c.SetIn(strings.NewReader(payload))
	c.SetOut(&out)
	c.SetErr(&strings.Builder{})
	_ = c.RunE(c, nil)
	return out.String()
}
