package tombstones

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A comment that is nothing but a tombstone loses its whole sentence, so the
// line goes with it rather than standing as bare punctuation.
func TestAWordingTellIsRewrittenOutOfSource(t *testing.T) {
	src := "// This used to read the flag.\nfunc f() {}\n"
	repair := Fix("a.go", src, DefaultMaxCommentLines)

	assert.True(t, repair.Changed)
	assert.Equal(t, "func f() {}\n", repair.Text)
	assert.Positive(t, repair.Rewrites)
	assert.Empty(t, repair.Kept)
}

// A comment sharing its line with code is left alone: the block is not prose
// end to end, and rewriting it would move the code beside it.
func TestACommentSharingACodeLineIsLeftAlone(t *testing.T) {
	src := "call() // previously the other one\n"
	repair := Fix("a.go", src, DefaultMaxCommentLines)

	assert.False(t, repair.Changed)
	assert.Equal(t, src, repair.Text)
	assert.Empty(t, repair.Kept)
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

// A document paragraph is prose too, so the table rewrites it. It comes back as
func TestADocumentParagraphIsRewritten(t *testing.T) {
	doc := "The loader previously read the flag. It now reads the file.\n"
	repair := Fix("notes.md", doc, DefaultMaxCommentLines)

	assert.True(t, repair.Changed)
	assert.Positive(t, repair.Rewrites)
	assert.NotContains(t, repair.Text, "previously")
	assert.Contains(t, repair.Text, "reads the file")
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

	// The reflow shortens it and the cap still names what survives, because a
	// block over the cap is over it by a thought rather than by padding.
	require.Len(t, repair.Kept, 1)
	assert.Contains(t, repair.Kept[0].Tell, "comment block of 5 lines")
	assert.NotContains(t, repair.Text, "\n\n", "no line was deleted")
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
