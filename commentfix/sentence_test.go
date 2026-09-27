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

// A single sentence too long for its code is cut at a clause boundary, never
// between words of the clause.
func TestALongSentenceIsCutAtAClause(t *testing.T) {
	for comment, want := range map[string]string{
		"\t// The pipes are ours, not StdoutPipe's: exec closes those at Wait, and this\n\t// process must outlive that to bound a reader itself. See drainGrace.\n":                  "// The pipes are ours, not StdoutPipe's: exec closes those at Wait.",
		"\t// Wait reports only once both spools have seen their end: reap closes the\n\t// relays after the grace, releasing a stream a grandchild still holds open.\n":                "// Wait reports only once both spools have seen their end: reap closes the relays after the grace.",
		"\t// A linked worktree's .git is a file naming a gitdir under the parent's\n\t// .git/worktrees; its refs and config live in the commondir beside it.\n":                     "// A linked worktree's .git is a file naming a gitdir under the parent's .git/worktrees.",
	} {
		src := "package p\n\nfunc f() {\n" + comment + "\tx := 1\n\t_ = x\n}\n"
		out, changed := FixLength("x.go", src)
		require.True(t, changed, comment)
		assert.Contains(t, out, want)
		assert.Empty(t, CheckLength("x.go", out))
	}
}

// A run whose prose never closes has no cut that reads, so every sentence-aware
// pass declines it and the force fit takes it instead: cut at a word, inside the
// budget, with the clause left dangling. That is the trade the force fit makes.
func TestARunThatNeverClosesIsForceFitted(t *testing.T) {
	body := strings.Repeat("// a clause that never closes and just keeps going onward\n", 8)
	src := "package p\n\n" + body + "const p = 1\n"

	require.NotEmpty(t, CheckLength("x.go", src))
	out, changed := FixLength("x.go", src)
	assert.True(t, changed)
	assert.Empty(t, CheckLength("x.go", out), "the force fit always lands inside the budget")
	assert.Contains(t, out, "// a clause that never closes")
	assert.Contains(t, out, "const p = 1", "the code it documents is untouched")
}
