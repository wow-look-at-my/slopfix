package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// A run whose prose never closes has no cut that reads. The repair leaves it as
// written and the check keeps reporting it, because a cut at a word would put a
// fragment in the author's mouth.
func TestARunThatNeverClosesIsLeftForAPerson(t *testing.T) {
	body := strings.Repeat("// a clause that never closes and just keeps going onward\n", 8)
	src := "package p\n\n" + body + "const p = 1\n"

	require.NotEmpty(t, CheckLength("x.go", src))
	out, _ := FixLength("x.go", src)
	assert.Equal(t, src, out, "no cut reads, so nothing is cut")
	assert.NotEmpty(t, CheckLength("x.go", out), "the finding stays for a person to rewrite")
}

// The opening sentence is never cut, even when it alone runs past the budget.
// The cut stops at its end, and the check reports what is still over.
func TestTheOpeningSentenceSurvivesWhole(t *testing.T) {
	src := "package p\n\n" +
		"// The first row is title plus tabs, and the next row is the per-tab subtitle\n" +
		"// (it is full-width in CSS, so it always wraps onto its own line). The\n" +
		"// subtitle length varies wildly per tab. Keeping it off the tab row is what\n" +
		"// pins the tab bar in place instead of letting it slide or wrap.\n" +
		"var head = 1\n"
	out, _ := FixLength("x.go", src)
	assert.Contains(t, out, "per-tab subtitle")
	assert.Contains(t, out, "wraps onto its own line).", "the opening sentence is whole")
	assert.NotContains(t, out, "per-tab.", "no fragment closed with a bolted-on period")
}
