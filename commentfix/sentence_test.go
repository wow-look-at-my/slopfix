package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/ste"
)

// The repair cuts at a sentence that ends, not at a line boundary. A comment is
// hard-wrapped, so a line cut strands a clause and can leave a bracket open.
func TestTheRepairCutsAtASentenceRatherThanALine(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// The port this listens on.",
		"// A deferral phrase often sits inside a question already quoted (\"Want me",
		"// to fix it?\" carries \"want me to\"), and naming both repeats the same",
		"// thing on the reader's screen twice over for no gain at all.",
		"const p = 1",
	}, "\n")

	out, changed := FixLength("x.go", src)
	require.True(t, changed)

	// What survives must not end mid-clause, and must not strand a bracket.
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		assert.Equal(t, strings.Count(line, "("), strings.Count(line, ")"),
			"a cut left an unclosed bracket: %q", line)
	}
	assert.Contains(t, out, "The port this listens on.")
	assert.NotContains(t, out, "(\"Want me")
	assert.Contains(t, out, "const p = 1")
}

func TestEndsSentenceReadsPastAClosingBracket(t *testing.T) {
	for _, line := range []string{
		"// It is the last word.",
		"// It is the last word (see above).",
		`// It is the last word."`,
		"// Is it the last word?",
		"// It is the last word!",
	} {
		assert.True(t, endsSentence(line), "%q ends a sentence", line)
	}

	for _, line := range []string{
		"// the clause runs on",
		"// a list follows:",
		"// it trails off ...",
		"// a shorthand such as e.g.",
	} {
		assert.False(t, endsSentence(line), "%q does not end a sentence", line)
	}
}

// The clause cuts are stated in rules/comment-clauses.xml. Each case is a
// comment placed above one line of code.
func TestEveryCommentClauseTestHolds(t *testing.T) {
	require.NotEmpty(t, clausesTable.Tests)

	for _, c := range clausesTable.Tests {
		t.Run(c.In, func(t *testing.T) {
			lines := reflow(c.In, "\t", "//", wrapWidth)
			src := "package p\n\nfunc f() {\n" + strings.Join(lines, "\n") + "\n\tx := 1\n\t_ = x\n}\n"
			out, changed := FixLength("x.go", src)
			require.True(t, changed)
			assert.Empty(t, CheckLength("x.go", out))

			var kept []string
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "//") {
					kept = append(kept, stripMarker(line))
				}
			}
			assert.Equal(t, c.Out, strings.Join(kept, " "))
		})
	}
}

// A run whose prose never closes has no cut that reads. No period is bolted onto
// a clause, and the check keeps reporting the block for a person to rewrite.
func TestARunThatNeverClosesIsCutAtAWord(t *testing.T) {
	body := strings.Repeat("// a clause that never closes and just keeps going onward\n", 8)
	src := "package p\n\n" + body + "const p = 1\n"

	require.NotEmpty(t, CheckLength("x.go", src))
	out, _ := FixLength("x.go", src)
	assert.Empty(t, CheckLength("x.go", out), "the repair leaves nothing to report:\n%s", out)
	assert.Contains(t, out, "// a clause that never closes", "the opening words survive")
}

// When no cut fits, what stays is whole sentences, repaired to STE and each
// under its word cap. A sentence is never cut mid-clause.
func TestWhatStaysIsWholeSTESentences(t *testing.T) {
	src := "package p\n\n" +
		"// The first row is title plus tabs, and the next row is the per-tab subtitle\n" +
		"// (it is full-width in CSS, so it always wraps onto its own line). The\n" +
		"// subtitle length varies wildly per tab. Keeping it off the tab row is what\n" +
		"// pins the tab bar in place instead of letting it slide or wrap.\n" +
		"var head = 1\n"
	out, changed := FixLength("x.go", src)
	require.True(t, changed)
	assert.NotContains(t, commentProse(out), "per-tab.", "no fragment closed with a bolted-on period")
	assertWholeSTESentences(t, out)
}

// assertWholeSTESentences checks what a length cut left: whole sentences, STE
// clean, each under the word cap.
func assertWholeSTESentences(t *testing.T, src string) {
	t.Helper()
	prose := commentProse(src)
	assert.True(t, endsSentence(prose), "the comment ends on a sentence end: %q", prose)
	for _, f := range ste.Check(prose, 1) {
		assert.Equal(t, ste.IDSentenceCap, f.ID, "what stays is STE apart from an undividable length: %q", prose)
	}
	for _, s := range ste.Sentences(prose) {
		if ste.WordCount(s) <= ste.SentenceWordCap {
			continue
		}
		_, divisible := ste.Leading(s)
		assert.False(t, divisible, "over the cap only where the parser finds no clause boundary: %q", s)
	}
}

// commentProse joins the prose of every line comment in src, directives aside.
func commentProse(src string) string {
	var words []string
	for _, line := range strings.Split(src, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "//")
		if !ok || strings.HasPrefix(rest, "go:") {
			continue
		}
		if rest = strings.TrimSpace(rest); rest != "" {
			words = append(words, rest)
		}
	}
	return strings.Join(words, " ")
}
