package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/ste"
)

// fixSentences runs the sentence fixer alone over src.
func fixSentences(filename, src string) string {
	f := fixer.Open(filename, src, fixer.Options{Kind: fixer.Source})
	fixer.Run(f, fixer.Named(FixerSentence))
	return f.Text()
}

// longFunction is a body long enough that no comment above it outweighs it.
func longFunction(name string) string {
	var body strings.Builder
	body.WriteString("func " + name + "() {\n")
	for range 60 {
		body.WriteString("\tstep()\n")
	}
	return body.String() + "}\n"
}

// A list item opens a sentence of its own, even after a row that ends on a
// colon. Read as one sentence, the lead-in and the items run past the cap.
func TestAListItemInACommentIsASentenceOfItsOwn(t *testing.T) {
	src := "package p\n\n" +
		"// With a backup launcher the slow response keeps streaming. The earliest of these to happen decides the race:\n" +
		"// - The original recovers above the floor or finishes: the backup stops.\n" +
		"// - The backup overtakes or finishes, or the original fails: it takes over.\n" +
		longFunction("Drive")
	assert.Empty(t, CheckSentences("p.go", src))
}

// A sentence with no clause boundary. Its trailing adverbial goes into a
// sentence of its own behind "This happens", whatever the comment weighs.
func TestALongSentenceWithNoClauseBoundaryDivides(t *testing.T) {
	src := "package p\n\n" +
		"// The loader reads every cached manifest from the shared store of the plugin cache in the home directory of the user on each start of a session in the editor window.\n" +
		longFunction("Load")
	assert.Empty(t, CheckLength("p.go", src), "the ratio check passes, so only the sentence check can report it")

	hits := CheckSentences("p.go", src)
	require.Len(t, hits, 1)
	assert.False(t, ste.ByHand(hits[0].Fix), hits[0].Fix)
	assert.Equal(t, 3, hits[0].Line)
	out := fixSentences("p.go", src)
	assert.Equal(t, "The loader reads every cached manifest from the shared store of the plugin cache in the home directory of the user. This happens on each start of a session in the editor window.", commentProse(out))
	assert.Empty(t, CheckSentences("p.go", out))
}

// A sentence of many words that joins clauses with "and". The repair divides
// it inside the comment, and each half is a whole sentence.
func TestALongSentenceDividesInsideTheComment(t *testing.T) {
	src := "package p\n\n" +
		"// The loader reads every cached manifest from the shared store and rebuilds the\n" +
		"// index of plugin hooks for each workspace the user opens in the editor during\n" +
		"// startup of the session.\n" +
		longFunction("Load")
	hits := CheckSentences("p.go", src)
	require.Len(t, hits, 1)
	assert.False(t, ste.ByHand(hits[0].Fix))
	assert.Equal(t, 3, hits[0].Line)
	assert.Equal(t, 5, hits[0].EndLine)

	out := fixSentences("p.go", src)
	assert.Equal(t, "The loader reads every cached manifest from the shared store. The loader rebuilds the index of plugin hooks for each workspace the user opens in the editor during startup of the session.", commentProse(out))
	assert.Contains(t, out, longFunction("Load"), "the code is untouched")
	assert.Empty(t, CheckSentences("p.go", out))
	assert.Equal(t, out, fixSentences("p.go", out), "a second pass changes nothing")
}

// Sentences from real comments. An empty want means the comment stays as written.
var realCommentSentences = []struct {
	name, comment, want string
}{
	{
		"a short sentence is no finding",
		"// The gate reads every file.",
		"",
	},
	{
		"a trailing adverbial after a so clause goes behind a carrier",
		"// Your task is to produce a faithful, concise summary of the conversation so far so that a successor assistant can continue the work seamlessly after the earlier turns are discarded.",
		"Your task is to produce a faithful, concise summary of the conversation so far so that a successor assistant can continue the work seamlessly. This happens after the earlier turns are discarded.",
	},
	{
		"a but clause divides with However",
		"// The gate reads every file that the session wrote, but if the cache is cold at the start of the run, the build waits for the whole tree.",
		"The gate reads every file that the session wrote. However, if the cache is cold at the start of the run, the build waits for the whole tree.",
	},
}

func TestARealCommentSentenceDividesOrStays(t *testing.T) {
	for _, c := range realCommentSentences {
		t.Run(c.name, func(t *testing.T) {
			src := "package p\n\n" + c.comment + "\n" + longFunction("Run")
			out := fixSentences("p.go", src)
			if c.want == "" {
				assert.Equal(t, src, out)
				return
			}
			assert.Equal(t, c.want, commentProse(out))
			assert.Empty(t, CheckSentences("p.go", out))
		})
	}
}

// A sentence whose only cut between words lands inside the noun phrase "log
// line". The repair divides it at a clause boundary or leaves it as written,
// and a sentence it leaves asks for a rewrite by hand.
func TestALongSentenceNeverDividesInsideANounPhrase(t *testing.T) {
	src := "package p\n\n" +
		"// String carries every number a filesystem was judged on, which is what lets a\n" +
		"// log line be answered without the daemon still running to be asked.\n" +
		longFunction("String")
	hits := CheckSentences("p.go", src)
	require.Len(t, hits, 1)

	out := fixSentences("p.go", src)
	assert.NotContains(t, out, "log. Line", out)
	assert.Contains(t, commentProse(out), "a log line be answered", out)
	if out == src {
		assert.True(t, ste.ByHand(hits[0].Fix), hits[0].Fix)
		return
	}
	assert.False(t, ste.ByHand(hits[0].Fix), hits[0].Fix)
	assert.Contains(t, commentProse(out), "judged on. ", out)
	assert.Empty(t, CheckSentences("p.go", out))
}

// A comment after code stays on its row. A block comment keeps its delimiters.
func TestADividedCommentKeepsItsShape(t *testing.T) {
	trailing := "package p\n\nvar x = 1 // The gate reads every file that the session wrote, but if the cache is cold at the start of the run, the build waits for the whole tree.\n"
	out := fixSentences("p.go", trailing)
	assert.Equal(t, 4, len(strings.Split(out, "\n")), out)
	assert.Contains(t, out, "var x = 1 // The gate reads every file that the session wrote. However, if")

	block := "int f(void);\n\n/* The gate reads every file that the session wrote, but if the cache is cold\n   at the start of the run, the build waits for the whole tree. */\nint g(void);\n"
	out = fixSentences("x.c", block)
	assert.Contains(t, out, "/* The gate reads every file that the session wrote. However, if")
	assert.Equal(t, 1, strings.Count(out, "*/"), out)
	assert.Contains(t, out, "*/\nint g(void);\n")
}

// A directive, a license notice and a doc code block are not prose.
func TestNoSentenceRuleReadsADirectiveOrALicense(t *testing.T) {
	long := "The loader reads every cached manifest from the shared store of the plugin cache in the home directory of the user on each start of a session in the editor window."
	for _, src := range []string{
		"//go:generate go run ./gen -- " + long + "\npackage p\n",
		"// Copyright the authors. Licensed under the Apache License, Version 2.0. " + long + "\npackage p\n",
		"package p\n\n// Example:\n//\n//\t" + long + "\nfunc F() {}\n",
	} {
		assert.Empty(t, CheckSentences("p.go", src), src)
	}
}
