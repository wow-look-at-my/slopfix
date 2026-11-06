package markdown_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/markdown"
)

func TestWordsOnlyRefusesAnAddedHyphen(t *testing.T) {
	assert.True(t, markdown.WordsOnly("one two\nthree", "one two three"))
	assert.False(t, markdown.WordsOnly("one two three", "one - two three"))
	assert.False(t, markdown.WordsOnly("one - two", "one two"))
}

func TestEntryRepairMarksOnlyEntryLines(t *testing.T) {
	src := "Some prose\nthat wraps.\n\nalpha\nbeta\n"
	edits := markdown.EntryRepair(src, markdown.EntryBullets)
	require.Len(t, edits, 1)
	assert.Equal(t, "Some prose\nthat wraps.\n\n- alpha\n- beta\n", markdown.Apply(src, edits, edit.Scope{}).Text)
}

func TestEntryRepairInBlankLineStyleAddsOnlyBlankLines(t *testing.T) {
	src := "alpha\nbeta\ngamma\n"
	edits := markdown.EntryRepair(src, markdown.EntryBlankLines)
	require.Len(t, edits, 1)
	assert.Equal(t, "alpha\n\nbeta\n\ngamma\n", markdown.Apply(src, edits, edit.Scope{}).Text)
}
