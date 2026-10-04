package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cut keeps the whole opening sentence, and drops the sentences after it.
func TestTheCutKeepsTheWholeOpeningSentence(t *testing.T) {
	src := "package p\n\ntype C struct {\n" +
		"\t// Progress, when set, is called by the sampler every progressTick with the\n" +
		"\t// current series' sample count and confidence interval. RunAll installs a\n" +
		"\t// live-line printer; tests leave it nil.\n" +
		"\tProgress func(samples int, rmePct, targetRMEPct float64)\n}\n"
	out, changed := FixLength("p.go", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("p.go", out))
	assert.Equal(t, "Progress, when set, is called by the sampler every progressTick with the current series' sample count and confidence interval.", commentProse(out))
}

// A comment over a literal element with a trailing comment is weighed against
// the element. Read as a comment row, the element measured as no code at all.
func TestAnElementWithATrailingCommentIsCode(t *testing.T) {
	src := "package p\n\nvar m = map[string]int{\n" +
		"\t\"a\": 1, // first\n" +
		"\t// 8-bit.\n" +
		"\t\"b\": 2, // second\n" +
		"}\n"
	assert.Empty(t, CheckLength("p.go", src), "a short heading over an element is in proportion")
	out, _ := FixLength("p.go", src)
	assert.Contains(t, out, "// 8-bit.", "nothing deletes it")
}
