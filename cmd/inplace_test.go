package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func inPlaceEdit(path, old, replacement string) map[string]any {
	return map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input":      map[string]any{"file_path": path, "old_string": old, "new_string": replacement},
	}
}

func onDisk(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// A line inside a fence is code. Judged as a fragment it reads as prose, and
// the repair collapses its alignment and the space before the dot.
func TestAnEditInsideAFenceIsJudgedWhereItLands(t *testing.T) {
	path := onDisk(t, "doc.md", "# Use\n\n```sh\nslopfix fmt docs/*.md     # join\n```\n")
	got := ask(t, inPlaceEdit(path, "slopfix fmt docs/*.md     # join", "slopfix purge .           # move"))
	if got.out != nil {
		assert.Nil(t, got.out["updatedInput"], "an edit inside a fence was rewritten")
	}
}

// Prose an edit brings in is still repaired, with the file around it.
func TestAnEditToProseIsRepairedWhereItLands(t *testing.T) {
	path := onDisk(t, "doc.md", "# Use\n\nThe gate is open.\n")
	got := ask(t, inPlaceEdit(path, "The gate is open.", "The gate is shut; the write fails."))
	require.NotNil(t, got.out)
	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated)
	assert.Equal(t, "The gate is shut. The write fails.", updated["new_string"])
}

// A repair that would reach past the edit changes text the write never
// touched, so none of it is applied.
func TestARepairThatReachesPastTheEditIsNotApplied(t *testing.T) {
	path := onDisk(t, "doc.md", "# Use\n\nIt doesn't hold.\n\nThe gate is open.\n")
	got := ask(t, inPlaceEdit(path, "The gate is open.", "The gate is shut; the write fails."))
	if got.out != nil {
		assert.Nil(t, got.out["updatedInput"])
	}
}
