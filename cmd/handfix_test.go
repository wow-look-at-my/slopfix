package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

const countsDoc = "Intro line.\nIt has three plugins.\nOutro line.\n"

// onDisk writes src to a real file and answers its path.
func onDisk(t *testing.T, name, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
	return path
}

func denial(t *testing.T, got answer) string {
	t.Helper()
	require.NotNil(t, got.out, "expected a refusal")
	require.Equal(t, "deny", got.out["permissionDecision"], got.body)
	reason, _ := got.out["permissionDecisionReason"].(string)
	return reason
}

func TestAnEditToAnAutoFixableLineIsDenied(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editPayload(path, "It has three plugins.", "It has three good plugins."), "counts")

	reason := denial(t, got)
	assert.Contains(t, reason, "run `slopfix fix "+path+"`, then make your change; hand-edits to lines slopfix auto-fixes are refused")
	assert.Contains(t, reason, path+":2: [")
	assert.Contains(t, reason, "counts")
}

func TestAnEditBesideAnAutoFixableLineIsAllowed(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editPayload(path, "Outro line.", "Closing line."), "counts")

	assert.NotContains(t, got.body, "deny")
}

func TestAnInsertBesideAnAutoFixableLineIsAllowed(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editPayload(path, "Outro line.\n", "Middle line.\nOutro line.\n"), "counts")

	assert.NotContains(t, got.body, "deny")
}

// A long comment block is flagged, and no rewrite of it lands.
func TestAnEditToAFindingFixDoesNotChangeIsAllowed(t *testing.T) {
	src := "package p\n"
	for i := range 40 {
		src += fmt.Sprintf("// The loader reads step %d of the file and returns the record it names.\n", i)
	}
	src += "func x() {}\n"
	path := onDisk(t, "a.go", src)
	got := ask(t, editPayload(path, "step 7 of", "stage 7 of"), "tombstones")

	assert.NotContains(t, got.body, "deny")
}

func TestAWriteReplacingAFlaggedLineIsDenied(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, write(path, "Intro line.\nIt has plugins.\nOutro line.\n"), "counts")

	reason := denial(t, got)
	assert.Contains(t, reason, path+":2: [")
}

func TestAWriteKeepingAFlaggedLineIsAllowed(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, write(path, "Intro line.\nIt has three plugins.\nClosing line.\n"), "counts")

	assert.NotContains(t, got.body, "deny")
}

func TestAMultiEditTouchingAnAutoFixableLineIsDenied(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	payload := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "MultiEdit",
		"tool_input": map[string]any{
			"file_path": path,
			"edits": []any{
				map[string]any{"old_string": "Intro", "new_string": "Opening"},
				map[string]any{"old_string": "line.", "new_string": "row.", "replace_all": true},
				map[string]any{"old_string": "three", "new_string": "3"},
			},
		},
	}
	reason := denial(t, ask(t, payload, "counts"))
	assert.Contains(t, reason, path+":2: [")
	assert.NotContains(t, reason, path+":1: [")
}

func TestAnEditTheToolCannotPlaceIsLeftToTheTool(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := ask(t, editPayload(path, "line.", "row."), "counts")

	assert.NotContains(t, got.body, "deny")
}

func TestAnErrorComputingTheFixIsDenied(t *testing.T) {
	dir := t.TempDir()
	got := ask(t, editPayload(dir, "a", "b"), "counts")

	reason := denial(t, got)
	assert.Contains(t, reason, "slopfix cannot compute what `slopfix fix "+dir+"` changes")
	assert.Contains(t, reason, "is a directory")
}

func TestAnUnreadableEditIsDenied(t *testing.T) {
	path := onDisk(t, "a.md", countsDoc)
	got := handFix("Edit", []byte(`{"replace_all":"yes"}`), path, "", nil, nil)

	require.NotEmpty(t, got)
	assert.Contains(t, got[0], "cannot read the edits to "+path)
}

func TestANewFileIsOnlyRepaired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.md")
	got := ask(t, write(path, "It has three plugins.\n"), "counts")

	require.NotNil(t, got.out)
	assert.NotContains(t, got.body, "deny")
	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated)
	assert.Equal(t, "It has plugins.\n", updated["content"])
}

// A line no finding names is named by the category whose repair changes it.
func TestALineIsNamedByTheCategoryThatChangesIt(t *testing.T) {
	names, err := ruleIDsOn(slopfix.Request{Content: countsDoc, Path: "a.md"}, []int{2})

	require.NoError(t, err)
	assert.NotEmpty(t, names[2])
}

func TestChangedLinesIgnoresAnInsertion(t *testing.T) {
	assert.Empty(t, changedLines("a\nb\n", "a\nx\nb\n"))
	assert.Equal(t, []int{2}, changedLines("a\nb\nc\n", "a\nB\nc\n"))
	assert.Equal(t, []int{1}, changedLines("a\nb\n", "b\n"))
}

func editPayload(path, old, new string) map[string]any {
	return map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input":      map[string]any{"file_path": path, "old_string": old, "new_string": new},
	}
}
