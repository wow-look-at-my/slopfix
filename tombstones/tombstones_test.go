package tombstones

import (
	"strings"
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

// A comment sharing its line with code is left alone. The block is not prose
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

// A document paragraph is prose too, so the table rewrites it.
func TestADocumentParagraphIsRewritten(t *testing.T) {
	doc := "The loader previously read the flag. It now reads the file.\n"
	repair := Fix("notes.md", doc, DefaultMaxCommentLines)

	assert.True(t, repair.Changed)
	assert.Positive(t, repair.Rewrites)
	assert.NotContains(t, repair.Text, "previously")
	assert.Contains(t, repair.Text, "reads the file")
}

// A rewrite keeps the indentation and marker that place a paragraph in its
// list. Without them a nested item leaves the list, and a paragraph under a
// numbered item ends that item.
func TestADocumentRewriteKeepsTheListStructure(t *testing.T) {
	doc := "  2. **Cached.** The loader previously read the flag.\n\n" +
		"     - the `access_tokens` mint\n" +
		"     - the loader previously read the rows\n" +
		"     - `branches`\n\n" +
		"     The tail previously read the flag.\n"
	repair := Fix("notes.md", doc, DefaultMaxCommentLines)

	assert.True(t, repair.Changed)
	assert.NotContains(t, repair.Text, "previously")
	assert.Equal(t, "  2. **Cached.** The loader read the flag.\n\n"+
		"     - the `access_tokens` mint\n"+
		"     - the loader read the rows\n"+
		"     - `branches`\n\n"+
		"     The tail read the flag.\n", repair.Text)
}

func TestAFencedBlockInADocumentIsNotProse(t *testing.T) {
	doc := "Read the flag.\n\n```\n// this used to read the flag\n```\n"
	repair := Fix("notes.md", doc, DefaultMaxCommentLines)

	assert.Equal(t, doc, repair.Text)
	assert.Empty(t, repair.Kept)
}

// A block over the cap with no sentence end to cut at keeps its opening
// sentence. The block is divided where it runs past the STE cap, and
// nothing after it.
func TestAVolumeCapWithNoSentenceEndKeepsItsOpening(t *testing.T) {
	src := ""
	for range 6 {
		src += "// the loader reads the flag and returns what it names\n"
	}
	repair := Fix("a.go", src, 3)
	assert.Empty(t, repair.Kept, repair.Text)
	lines := strings.Split(strings.TrimRight(repair.Text, "\n"), "\n")
	require.LessOrEqual(t, len(lines), 3, repair.Text)
	assert.True(t, strings.HasSuffix(lines[len(lines)-1], "."), repair.Text)
	assert.True(t, strings.HasPrefix(repair.Text, "// the loader reads the flag"), repair.Text)
}

// A block over the cap whose lines end sentences loses whole lines from its end.
func TestTheVolumeCapDropsLinesToASentenceEnd(t *testing.T) {
	src := "/* The loader reads the flag.\n *\n"
	for range 4 {
		src += " *   <x-widget>save</x-widget>\n"
	}
	src += " */\nint x;\n"
	repair := Fix("a.c", src, 3)
	assert.Empty(t, repair.Kept, repair.Text)
	assert.Contains(t, repair.Text, "The loader reads the flag.", repair.Text)
	assert.NotContains(t, repair.Text, "x-widget", repair.Text)
	assert.Equal(t, strings.Count(repair.Text, "/*"), strings.Count(repair.Text, "*/"), repair.Text)
}

// A block over the cap that ends its sentences loses its last thoughts.
func TestTheVolumeCapCutsWholeSentences(t *testing.T) {
	src := ""
	for i := range 6 {
		src += "// The loader reads flag " + strings.Repeat("x", i+1) + " and returns what it names.\n"
	}
	repair := Fix("a.go", src, 3)
	assert.Empty(t, repair.Kept)
	assert.Contains(t, repair.Text, "// The loader reads flag x and returns what it names.")
	assert.NotContains(t, repair.Text, "xxxxxx")
}

// A note at the end of each code line is read with that line, not as a block.
func TestTrailingCommentsAreNotAVolume(t *testing.T) {
	src := "func f() {\n\t// the flags the sequence must hold\n"
	for range 6 {
		src += "\tcheck(sequence, flag) // one flag the sequence holds\n"
	}
	src += "}\n"
	repair := Fix("a.go", src, 3)

	assert.Empty(t, repair.Kept)
	assert.Equal(t, src, repair.Text)
}

// A run of //sys directives is a generator's input, not prose, so it never
// reaches the volume cap however long it runs.
func TestDirectiveRunsAreNotAVolume(t *testing.T) {
	src := "package syscall\n\n"
	for range 6 {
		src += "//sys\tGetpid() (pid int)\n"
	}
	repair := Fix("syscall_cosmo.go", src, 3)

	assert.Empty(t, repair.Kept)
	assert.Equal(t, src, repair.Text)
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
