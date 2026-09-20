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

// clean is the control: the same file with nothing to report.
const cleanFile = `package p

// The walk visits each node.
func walk() {}
`

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

// The case this exists for: the comment is reworded, the code is untouched, and
// the file is one the rules report.
func TestARewordedCommentIsRefusedInAReportedFile(t *testing.T) {
	path := onDisk(t, "a.go", reported)
	out := drive(t, editPayload(t, path, "// The walk has 3 phases.", "// The walk runs in phases."))

	require.NotEmpty(t, out, "a reworded comment must not pass in a file slopfix reports")
	assert.Contains(t, out, `"permissionDecision":"deny"`)
	assert.Contains(t, out, "slopfix fix")
}

// The control that proves the case above can fail: the same reword, in a file
// the rules read cleanly, is ordinary work.
func TestARewordedCommentIsAllowedInACleanFile(t *testing.T) {
	path := onDisk(t, "a.go", cleanFile)
	out := drive(t, editPayload(t, path, "// The walk visits each node.", "// The walk reaches every node."))

	assert.NotContains(t, out, `"permissionDecision":"deny"`)
}

// An edit that moves code is not this rule's business, whatever it does to the
// comment beside it.
func TestAnEditThatMovesCodeIsAllowed(t *testing.T) {
	path := onDisk(t, "a.go", reported)
	out := drive(t, editPayload(t,
		path,
		"// The walk has 3 phases.\nfunc walk() {}",
		"// The walk runs in phases.\nfunc walk() error { return nil }"))

	assert.NotContains(t, out, `"permissionDecision":"deny"`)
}

// Writing a NEW comment is ordinary work, so an addition is allowed even where
// the file is reported.
func TestAddingACommentIsAllowed(t *testing.T) {
	path := onDisk(t, "a.go", reported)
	out := drive(t, editPayload(t, path, "func walk() {}", "// walk reaches every node.\nfunc walk() {}"))

	assert.NotContains(t, out, `"permissionDecision":"deny"`)
}

// A language with no grammar here cannot be split into code and comment, so it
// is never refused on this rule.
func TestAnUnparsedLanguageIsAllowed(t *testing.T) {
	path := onDisk(t, "notes.txt", "# The walk has 3 phases.\n")
	out := drive(t, editPayload(t, path, "# The walk has 3 phases.", "# The walk runs in phases."))

	assert.NotContains(t, out, `"permissionDecision":"deny"`)
}

// Every uncertainty allows the write: a file that is not there, and a
// replacement that does not appear exactly once.
func TestEveryUncertaintyIsAllowed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone.go")
	assert.NotContains(t, drive(t, editPayload(t, missing, "// a", "// b")), `"permissionDecision":"deny"`)

	twice := onDisk(t, "a.go", "package p\n\n// 3 phases.\nfunc a() {}\n\n// 3 phases.\nfunc b() {}\n")
	assert.NotContains(t, drive(t, editPayload(t, twice, "// 3 phases.", "// several phases.")), `"permissionDecision":"deny"`)
}
