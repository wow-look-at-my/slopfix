package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// blockOverTheCap is a /* */ block over the volume cap whose */ closes its own
// last paragraph, above a second block and an import.
func blockOverTheCap() string {
	var src strings.Builder
	src.WriteString("/* <scratch-widget> is the first widget of the kit.\n *\n")
	for range 12 {
		src.WriteString(" * The widget seals its structure and styles so nobody can assemble it wrong.\n")
	}
	src.WriteString(" *\n *   <scratch-widget>save</scratch-widget>\n *   <scratch-widget variant=\"accent\">add</scratch-widget>\n */\n\n")
	src.WriteString("/* A single stylesheet is shared by every instance. */\n\n")
	src.WriteString("import { SHEET } from './styles.ts';\n\nexport const widget = SHEET;\n")
	return src.String()
}

// The volume cut drops a /* */ block's last paragraph and keeps the block closed.
// An open block runs into the comment under it, and the next cut can delete code.
func TestCapLinesKeepsABlockClosed(t *testing.T) {
	text := strings.Split(strings.TrimSuffix(strings.SplitAfter(blockOverTheCap(), " */\n")[0], "\n"), "\n")
	require.Greater(t, len(text), tombstones.DefaultMaxCommentLines, "the fixture must be over the cap")
	kept := commentfix.CapLines(text, tombstones.DefaultMaxCommentLines)
	require.NotEmpty(t, kept)
	assert.LessOrEqual(t, len(kept), tombstones.DefaultMaxCommentLines, "the closer line counts against the cap:\n%s", strings.Join(kept, "\n"))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(kept[len(kept)-1]), "*/"), "the block ends open:\n%s", strings.Join(kept, "\n"))
	assert.Equal(t, 1, strings.Count(strings.Join(kept, "\n"), "*/"), strings.Join(kept, "\n"))
}

// Every rule together leaves each /* */ block closed, and the code untouched.
func TestFixLeavesEveryBlockClosed(t *testing.T) {
	src := blockOverTheCap()
	repair := slopfix.Fix(slopfix.Request{Content: src, Path: "widget.ts", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	assert.Equal(t, strings.Count(repair.Text, "/*"), strings.Count(repair.Text, "*/"), "a block lost its closer:\n%s", repair.Text)
	assert.Contains(t, repair.Text, "import { SHEET } from './styles.ts';\n\nexport const widget = SHEET;\n", "the code is untouched")
	assert.Contains(t, repair.Text, "/* <scratch-widget> is the first widget of the kit.", "the opening survives:\n%s", repair.Text)
	assert.Empty(t, commentfix.CheckLength("widget.ts", repair.Text), "a cut fits, so none is left:\n%s", repair.Text)
	for _, hit := range repair.Kept {
		assert.NotContains(t, []string{commentfix.IDLength, tombstones.IDVolume}, hit.ID, "%s is left:\n%s", hit.ID, repair.Text)
	}
}
