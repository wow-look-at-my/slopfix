package tombstones

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWordingTellIsStrippedOutOfSource(t *testing.T) {
	src := "// This used to read the flag.\nfunc f() {}\n"
	repair := Fix("a.go", src, DefaultMaxCommentLines)

	assert.True(t, repair.Changed)
	assert.Equal(t, "func f() {}\n", repair.Text)
	require.Len(t, repair.Removed, 1)
	assert.Contains(t, repair.Removed[0], "used to read")
	assert.Empty(t, repair.Kept)
}

func TestACommentSharingACodeLineIsKeptRatherThanGuessedAt(t *testing.T) {
	src := "call() // previously the other one\n"
	repair := Fix("a.go", src, DefaultMaxCommentLines)

	assert.False(t, repair.Changed)
	assert.Equal(t, src, repair.Text)
	require.Len(t, repair.Kept, 1)
	assert.Equal(t, "a former state", repair.Kept[0].Tell)
}

func TestOrdinaryProseInSourceSurvives(t *testing.T) {
	src := "// f reads the flag and returns what it names.\nfunc f() {}\n"
	repair := Fix("a.go", src, DefaultMaxCommentLines)

	assert.False(t, repair.Changed)
	assert.Equal(t, src, repair.Text)
	assert.Empty(t, repair.Kept)
}

func TestCodeIsNeverJudged(t *testing.T) {
	src := "msg := \"this used to work\"\n"
	repair := Fix("a.go", src, DefaultMaxCommentLines)

	assert.Equal(t, src, repair.Text)
	assert.Empty(t, repair.Kept)
}

func TestADocumentFindingIsReportedRatherThanStripped(t *testing.T) {
	doc := "The loader previously read the flag. It now reads the file.\n"
	repair := Fix("notes.md", doc, DefaultMaxCommentLines)

	assert.False(t, repair.Changed)
	assert.Equal(t, doc, repair.Text)
	require.NotEmpty(t, repair.Kept)
	assert.False(t, repair.Kept[0].Strippable)
}

func TestAFencedBlockInADocumentIsNotProse(t *testing.T) {
	doc := "Read the flag.\n\n```\n// this used to read the flag\n```\n"
	repair := Fix("notes.md", doc, DefaultMaxCommentLines)

	assert.Equal(t, doc, repair.Text)
	assert.Empty(t, repair.Kept)
}

func TestTheVolumeCapReportsRatherThanStrips(t *testing.T) {
	src := ""
	for range 6 {
		src += "// the loader reads the flag and returns what it names\n"
	}
	repair := Fix("a.go", src, 3)

	assert.False(t, repair.Changed)
	require.Len(t, repair.Kept, 1)
	assert.Contains(t, repair.Kept[0].Tell, "comment block of 6 lines")
}

func TestAnUnjudgedPathIsLeftAlone(t *testing.T) {
	src := "this used to work\n"
	repair := Fix("a.bin", src, DefaultMaxCommentLines)

	assert.Equal(t, src, repair.Text)
	assert.Empty(t, repair.Kept)
}

func TestIdentifierShapeRefusesAWordAndAcceptsASymbol(t *testing.T) {
	assert.False(t, isCandidate("previously"))
	assert.False(t, isCandidate("ENOSYS"))
	assert.False(t, isCandidate("reader"))
	assert.True(t, isCandidate("TestDarwinStatfsToLinux"))
	assert.True(t, isCandidate("comment_blocks"))
}
