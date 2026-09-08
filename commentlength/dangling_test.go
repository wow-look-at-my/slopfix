package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A cut lands on a sentence end wherever the prose has one.
//
// The repair used to cut whole lines, so a line holding the end of a sentence
// and the start of the next was kept entire. "The grammar also declares an
// error-recovery token. This scanner never" shipped that way. A dangling clause
// is a worse comment than the long comment it replaced.
func TestARepairCutsAtASentenceEndInsideALine(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// The grammar declares a token this scanner never produces. The blank",
		"// below it keeps every index above right, because the parser reads them",
		"// by position rather than by name.",
		"const p = 1",
	}, "\n")

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Contains(t, out, "The grammar declares a token this scanner never produces.")
	assert.NotContains(t, out, "The blank")
	assert.Empty(t, Check("x.go", out))
}

// Prose that carries no sentence end at all is still shortened. The line cut is
// the last resort, and losing the tail beats leaving the essay whole.
func TestProseWithNoSentenceEndIsStillShortened(t *testing.T) {
	var b strings.Builder
	for range 6 {
		b.WriteString("// an explanation that runs on well past the declaration below it,\n")
	}
	src := "package p\n\n" + b.String() + "const p = 1\n"

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Less(t, len(out), len(src))
	assert.Empty(t, Check("x.go", out))
}

// A block that holds an earlier ending is repaired at that ending, which is the
// control that proves the cut is not simply the last line going.
func TestABlockWithAnEarlierEndingIsRepairedThere(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// The bound every caller shares.",
		"// It was raised, and the reason is that the old value truncated a payload",
		"// nobody had measured, which took a week of somebody's time.",
		"const p = 1",
	}, "\n")

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Contains(t, out, "The bound every caller shares.")
	assert.NotContains(t, out, "took a week")
	assert.Empty(t, Check("x.go", out))
}
