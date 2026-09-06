package slopfmt_test

import (
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

// What a rewrite cannot repair is reported rather than guessed at.
func TestFixReportsWhatItCannotRepair(t *testing.T) {
	repair := slopfmt.Fix("It doesn't expand its contraction.\n")
	require.NotEmpty(t, repair.Findings)
	assert.Contains(t, repair.Findings[0].Fix, "does not")
}

func TestFixReportsTheFindingsOfTheRepairedText(t *testing.T) {
	repair := slopfmt.Fix("A sentence that is wrapped\nand that doesn't expand.\n")
	assert.True(t, repair.Changed)
	require.Len(t, repair.Findings, 1)
	assert.Equal(t, 1, repair.Findings[0].Line)
}
