package commentfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/commentfix"
)

// header is the package clause a fixture needs to parse as Go.
const header = "package p\n\n"

// fix repairs a fixture and hands back the repair with the header off, so a
// case reads as the snippet it is about.
func fix(t *testing.T, src string) commentfix.Repair {
	t.Helper()
	repair := commentfix.Fix("x.go", header+src)
	repair.Text = strings.TrimPrefix(repair.Text, header)
	return repair
}

// The property the rule exists for: what Fix writes carries no finding. A
// repair that leaves the rule reporting is not a repair.
func TestWhatFixWritesCarriesNoFinding(t *testing.T) {
	for _, src := range []string{
		"// It reserves two slots with three atomic adds.\nfunc f() {}\n",
		"// The other two run a loop.\ntype T struct{}\n",
		"// Two goroutines never wait for one another.\nfunc g() {}\n",
		"// Each shard is padded to 128 bytes.\nvar x int\n",
	} {
		repair := fix(t, src)
		require.True(t, repair.Changed, "nothing repaired in %q", src)
		assert.Empty(t, commentfix.Check("x.go", header+repair.Text),
			"a finding survived the repair of %q: %q", src, repair.Text)
	}
}

// The table says it in words wherever a swap keeps the meaning, so the sentence
// survives the repair rather than being cut.
func TestATableEntryKeepsTheSentence(t *testing.T) {
	repair := fix(t, "// It reserves two slots with three atomic adds.\nfunc f() {}\n")
	assert.Equal(t, "// It reserves slots with atomic adds.\nfunc f() {}\n", repair.Text)
	assert.Empty(t, repair.Removed, "a rewritten sentence is not a cut one")
}

// The whole repair, cut and residual pass included, over the comments a run
// over gh-wait-ci wrecked. Only the tallies change, and no sentence is cut.
func TestTheWholeRepairChangesOnlyTheTallies(t *testing.T) {
	src := "// It fails if exactly one is found.\n" +
		"// It picks the flag, otherwise the one the current directory names.\n" +
		"// The one thing that matters most.\n" +
		"// A workflow job is one of these, but so is a status.\n" +
		"// Splits once, before any subcommand runs.\n" +
		"// It lists workflow runs, newest first.\n" +
		"// A zero deadline never expires.\n" +
		"//\n" +
		"// It names the three shapes GitHub uses.\n" +
		"// It lists them, so the two are kept apart.\nfunc f() {}\n"
	want := strings.Replace(src, "the three shapes", "the shapes", 1)
	want = strings.Replace(want, "so the two are", "so both are", 1)

	repair := fix(t, src)
	assert.Empty(t, repair.Removed, "no sentence is cut")
	assert.Empty(t, commentfix.Check("x.go", header+repair.Text))
	// A paragraph rewrite rewraps, so the words are compared rather than the lines.
	assert.Equal(t, strings.Fields(want), strings.Fields(repair.Text))
}

// A tally no entry covers is not guessed at. The sentence goes, and the
// caller is told which sentence went.
func TestANumberNoEntryCoversCutsItsSentence(t *testing.T) {
	repair := fix(t, "// It is padded. Each shard is padded to 128 bytes.\nvar x int\n")
	assert.Equal(t, "// It is padded.\nvar x int\n", repair.Text)
	assert.Equal(t, []string{"Each shard is padded to 128 bytes."}, repair.Removed)
}

// A comment left with nothing to say loses its line rather than sitting there
// as a bare marker.
func TestACommentLeftWithNothingToSayLosesItsLine(t *testing.T) {
	repair := fix(t, "// Each shard is padded to 128 bytes.\nvar x int\n")
	assert.Equal(t, "var x int\n", repair.Text)
}

// A sentence wraps across comment lines, and a cut takes the whole sentence.
// Cutting only the share a line carries leaves the rest dangling below it,
// which is what the repair did before it read a paragraph at a time.
func TestACutTakesAWrappedSentenceWhole(t *testing.T) {
	src := "// A take that finds it empty puts refillBatch values back. Every\n" +
		"// measured take therefore also pays for 1 add. Subtract the add\n" +
		"// benchmark to isolate the take itself.\nfunc f() {}\n"

	repair := fix(t, src)
	assert.NotContains(t, repair.Text, "pays for", "the sentence carrying the number goes whole")
	assert.NotContains(t, repair.Text, "// add.", "no fragment of it is left behind")
	assert.Contains(t, repair.Text, "puts refillBatch values back.")
	assert.Contains(t, repair.Text, "Subtract the add")
	assert.Equal(t, []string{"Every measured take therefore also pays for 1 add."}, repair.Removed)
	assert.Empty(t, commentfix.Check("x.go", header+repair.Text))
}

// A rewrite of a wrapped paragraph keeps the prose on its own lines rather
// than running it together.
func TestARewrittenParagraphKeepsItsShape(t *testing.T) {
	src := "// Bag.AddRange links the whole batch with one compare-and-swap, and\n" +
		"// the other two run a loop instead of a single atomic write.\nfunc f() {}\n"

	repair := fix(t, src)
	for _, line := range strings.Split(repair.Text, "\n") {
		assert.LessOrEqual(t, len(line), 80, "the repair wrapped at the width the paragraph had")
	}
	assert.Contains(t, repair.Text, "the others run a loop", "a cardinal standing in for a noun is said, not cut")
	assert.Empty(t, repair.Removed)
	assert.Empty(t, commentfix.Check("x.go", header+repair.Text))
}

// A comment following code on its line is repaired too, and the code in front
// of it is not prose the rewrite may touch.
func TestACommentFollowingCodeIsRepaired(t *testing.T) {
	repair := fix(t, "func f() {\n\tb.CompleteAdding() // Two calls change nothing.\n}\n")
	assert.Equal(t, "func f() {\n\tb.CompleteAdding() // Calls change nothing.\n}\n", repair.Text)
	assert.Empty(t, commentfix.Check("x.go", header+repair.Text))
}

// When the cut takes all of it, the code keeps its line and loses the comment.
func TestACutTrailingCommentLeavesTheCode(t *testing.T) {
	repair := fix(t, "func f() {\n\ts.Remove(2, 5) // 5 was never present\n}\n")
	assert.Equal(t, "func f() {\n\ts.Remove(2, 5)\n}\n", repair.Text)
	assert.Empty(t, commentfix.Check("x.go", header+repair.Text))
}

// A blank comment line the source already carried is a paragraph break somebody
// wrote.
func TestABlankCommentLineTheSourceCarriedSurvives(t *testing.T) {
	src := "// It locks.\n//\n// It reserves two slots.\nfunc f() {}\n"
	repair := fix(t, src)
	assert.Equal(t, "// It locks.\n//\n// It reserves slots.\nfunc f() {}\n", repair.Text)
}

// The negative control. A file the rule reports nothing in is written back
// byte for byte, so the cases above pass on a repair rather than on any edit.
func TestAFileWithNoFindingIsUntouched(t *testing.T) {
	src := "// It reserves a slot and publishes it.\nfunc f() {}\n"
	repair := fix(t, src)
	assert.False(t, repair.Changed)
	assert.Equal(t, src, repair.Text)
}

// A generated file is left alone, the same way the check skips it.
func TestAGeneratedFileIsLeftAlone(t *testing.T) {
	src := "// Code generated by hand. DO NOT EDIT.\n\n// It reserves two slots.\npackage p\n"
	repair := commentfix.Fix("x.go", src)
	assert.False(t, repair.Changed)
	assert.Equal(t, src, repair.Text)
}

// A directive addresses a tool rather than a reader, so the repair leaves it
// alone.
func TestADirectiveIsNotRewritten(t *testing.T) {
	src := "//go:build two\n\n// It reserves two slots.\npackage p\n"
	repair := commentfix.Fix("x.go", src)
	assert.Contains(t, repair.Text, "//go:build two")
	assert.Contains(t, repair.Text, "// It reserves slots.")
}

// Code is not prose. A number in a string literal or an expression is the
// program, and the repair never reaches it.
func TestCodeIsNotRewritten(t *testing.T) {
	src := "func f() int {\n\tconst two = 2\n\treturn two + 1\n}\n"
	repair := fix(t, src)
	assert.False(t, repair.Changed)
	assert.Equal(t, src, repair.Text)
}

// The rule reads every language the extractor knows, so the repair does too.
func TestTheRepairFollowsTheExtractorIntoAnotherLanguage(t *testing.T) {
	repair := commentfix.Fix("x.sh", "# It reserves two slots.\necho hi\n")
	assert.Equal(t, "# It reserves slots.\necho hi\n", repair.Text)
}

// A block comment is a single token spanning its lines. The repair read only
// the line it opens on, so a number below the opener was reported for ever and
// no run could clear it.
func TestABlockCommentIsRepairedBelowItsOpener(t *testing.T) {
	src := "int a;\n\n/* Keeps the ring.\n * The tables run to 12 sections. */\nint b;\n"
	got := commentfix.Fix("x.c", src)

	assert.True(t, got.Changed)
	assert.Contains(t, got.Text, "Keeps the ring.")
	assert.NotContains(t, got.Text, "12")
	assert.Empty(t, commentfix.Check("x.c", got.Text), "nothing is left to report")
}

// The closer is not prose. Dropped, the comment stays open and every
// declaration below it is swallowed by it, so an emptied block keeps its
// delimiters and the file still parses.
func TestAnEmptiedBlockKeepsItsDelimiters(t *testing.T) {
	src := "int a;\n\n/* The tables run to 12 sections. */\nint b;\n"
	got := commentfix.Fix("x.c", src)

	assert.True(t, got.Changed)
	assert.Contains(t, got.Text, "*/", "the block is closed")
	assert.Contains(t, got.Text, "int b;")
	assert.Equal(t, strings.Count(src, "/*"), strings.Count(got.Text, "/*"), "openers are balanced")
	assert.Equal(t, strings.Count(src, "*/"), strings.Count(got.Text, "*/"), "closers are balanced")
	assert.Empty(t, commentfix.Check("x.c", got.Text))
}

// A block opens a single time. Repeating its opener down the paragraph nests a
// comment inside itself, which is a syntax error in C.
func TestARewrittenBlockDoesNotRepeatItsOpener(t *testing.T) {
	long := "/* Asked once. " + strings.Repeat("A clause that carries the paragraph well past a line. ", 4) + "*/\n"
	src := "int a;\n\n" + long + "int b;\n"
	got := commentfix.Fix("x.c", src)

	require.True(t, got.Changed)
	assert.Equal(t, 1, strings.Count(got.Text, "/*"), "the opener is written a single time")
	assert.Equal(t, 1, strings.Count(got.Text, "*/"))
}

// A blank line inside a block comment breaks the prose, not the comment. Split
// into paragraphs, every half got a closer and each half past the opener
// began a comment nothing closed, so the C file stopped compiling.
func TestABlockCommentWithABlankLineStaysOneComment(t *testing.T) {
	src := "int a;\n\n/* Keeps the ring, and says how.\n *\n * The tables run to 12 sections. */\nint b;\n"
	got := commentfix.Fix("x.c", src)

	require.True(t, got.Changed)
	assert.Equal(t, 1, strings.Count(got.Text, "/*"), "the block still opens a single time")
	assert.Equal(t, 1, strings.Count(got.Text, "*/"), "and closes a single time")
	assert.Contains(t, got.Text, "int b;")
	assert.Empty(t, commentfix.Check("x.c", got.Text))
}

// An indented example or a table inside a block carries no marker of its own,
// and a rewrap would destroy it. The paragraph rewrite therefore declines the
// block, and the number is deleted where it sits instead: the table keeps its
// shape and the rule is left with nothing to report.
func TestABlockHoldingUnmarkedLinesKeepsItsShape(t *testing.T) {
	src := "int a;\n\n/* Layout, in 3 parts:\n\n     a | b\n\n */\nint b;\n"
	got := commentfix.Fix("x.c", src)

	assert.Contains(t, got.Text, "a | b", "the table survives")
	assert.Equal(t, strings.Count(src, "*/"), strings.Count(got.Text, "*/"))
	assert.Empty(t, commentfix.Check("x.c", got.Text))
}

// A block whose closer sits on a line of its own. The marker scan read that
// line's star as a continuation and its slash as prose, so the delimiter was
// lost, a stray byte entered the text, and the block was declined instead.
func TestABlockWhoseCloserHasItsOwnLineIsRepaired(t *testing.T) {
	src := "int a;\n\n/* Keeps the ring.\n * The tables run to 12 sections.\n */\nint b;\n"
	got := commentfix.Fix("x.c", src)

	require.True(t, got.Changed)
	assert.NotContains(t, got.Text, "12")
	assert.Equal(t, 1, strings.Count(got.Text, "/*"))
	assert.Equal(t, 1, strings.Count(got.Text, "*/"))
	assert.NotContains(t, got.Text, "sections. /", "the closer is not prose")
	assert.Empty(t, commentfix.Check("x.c", got.Text))
}
