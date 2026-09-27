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

// A run whose prose never closes has no cut that reads. It stays as written,
// and the finding goes to a person.
func TestARunThatNeverClosesIsLeftForAPerson(t *testing.T) {
	body := strings.Repeat("// a clause that never closes and just keeps going onward\n", 8)
	src := "package p\n\n" + body + "const p = 1\n"

	hits := CheckLength("x.go", src)
	require.Len(t, hits, 1)
	assert.False(t, hits[0].Repairable)
	out, changed := FixLength("x.go", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
}
