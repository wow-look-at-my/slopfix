package cmd

import (
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

// A repair the file needs outside the edit is text the write never touched, so
// it never lands. The edit's own text is still repaired.
func TestARepairOutsideTheEditIsLeftForItsAuthor(t *testing.T) {
	path := onDisk(t, "doc.md", "# Use\n\nIt doesn't hold.\n\nThe gate is open.\n")
	got := ask(t, inPlaceEdit(path, "The gate is open.", "The gate is shut; the write fails."))
	require.NotNil(t, got.out)
	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated)
	assert.Equal(t, "The gate is shut. The write fails.", updated["new_string"])
}

// An edit that lands inside a paragraph cannot have the paragraph rejoined,
// because the join rewrites words the write never touched.
func TestAnEditInsideAParagraphLeavesTheParagraphAlone(t *testing.T) {
	path := onDisk(t, "doc.md", "# Use\n\nThe gate is open\nand it doesn't close.\n")
	got := ask(t, inPlaceEdit(path, "The gate is open", "The gate is shut"))
	if got.out != nil {
		assert.Nil(t, got.out["updatedInput"])
	}
}
