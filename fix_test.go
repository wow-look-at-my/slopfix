package slopfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfmt"
)

func TestFixJoinsAWrapAndCutsACount(t *testing.T) {
	repair := slopfmt.Fix("There are three sections in the payload,\nand each one is read.\n")
	assert.True(t, repair.Changed)
	assert.Equal(t, "There are sections in the payload, and each one is read.\n", repair.Text)
	assert.Equal(t, []string{"three sections"}, repair.Removed)
}

func TestFixLeavesACleanDocumentAlone(t *testing.T) {
	doc := "A paragraph that needs no repair at all.\n"
	repair := slopfmt.Fix(doc)
	assert.False(t, repair.Changed)
	assert.Equal(t, doc, repair.Text)
	assert.Empty(t, repair.Removed)
	assert.Empty(t, repair.Findings)
}

// A fence is data. Joining its lines changes what the code says.
func TestFixKeepsAFencedBlockWhole(t *testing.T) {
	doc := "# Title\n\n```sh\nfirst\nsecond\n```\n"
	repair := slopfmt.Fix(doc)
	assert.False(t, repair.Changed)
	assert.Equal(t, doc, repair.Text)
}

// What a rewrite cannot repair is reported rather than guessed at. Splitting a
// long sentence needs a writer who knows which half is the point.
func TestFixReportsWhatItCannotRepair(t *testing.T) {
	long := "The gate reads every file in the session and refuses the write when any one of them carries a finding that a rewrite cannot repair on its own.\n"
	repair := slopfmt.Fix(long)
	assert.False(t, repair.Changed)
	require.NotEmpty(t, repair.Findings)
	assert.Contains(t, repair.Findings[0].Fix, "Split it")
}

// The prose repair runs on the joined paragraph, so a rule sees the sentence
// the hand wrap cut in two.
func TestFixRepairsProseAcrossAWrap(t *testing.T) {
	repair := slopfmt.Fix("A sentence that is wrapped\nand that doesn't expand.\n")
	assert.True(t, repair.Changed)
	assert.Equal(t, "A sentence that is wrapped and that does not expand.\n", repair.Text)
	assert.Empty(t, repair.Findings)
}

func TestFixRepairsTheMechanicalRules(t *testing.T) {
	repair := slopfmt.Fix("It doesn't matter; a caller should wait, so the write fails.\n")
	assert.True(t, repair.Changed)
	assert.Equal(t, "It does not matter. A caller must wait. So the write fails.\n", repair.Text)
	assert.Empty(t, repair.Findings)
}

// A list item is prose, and its marker survives the repair.
func TestFixKeepsAListMarker(t *testing.T) {
	repair := slopfmt.Fix("- It doesn't run.\n")
	assert.Equal(t, "- It does not run.\n", repair.Text)
}

// A fence is data, so no prose rule reaches inside it.
func TestFixLeavesProseInsideAFenceAlone(t *testing.T) {
	doc := "```sh\nit doesn't run; nothing does\n```\n"
	repair := slopfmt.Fix(doc)
	assert.False(t, repair.Changed)
	assert.Equal(t, doc, repair.Text)
}

func TestFixFileWritesTheRepair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("It doesn't run.\n"), 0o644))

	repair, err := slopfmt.FixFile(path)
	require.NoError(t, err)
	assert.True(t, repair.Changed)

	written, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "It does not run.\n", string(written))
}

func TestFixFileLeavesACleanFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("A clean line.\n"), 0o400))

	repair, err := slopfmt.FixFile(path)
	require.NoError(t, err)
	assert.False(t, repair.Changed)
}
