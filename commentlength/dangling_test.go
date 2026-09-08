package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A cut lands on a sentence end or it does not happen.
//
// The repair used to lop a line at a time once no paragraph break and no
// earlier sentence ending were left. That left the opening line's own sentence
// half written, so "This scanner never" and "A colon after the" both shipped.
// A dangling clause is a worse comment than the long one it replaced.
func TestARepairNeverLeavesAHalfWrittenSentence(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// The grammar declares a token this scanner never produces, and the blank",
		"// below keeps every index above it right, which matters because the parser",
		"// reads them positionally rather than by name and has no way to notice.",
		"const p = 1",
	}, "\n")

	out, _ := Fix("x.go", src)
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		assert.True(t, endsSentence(line) || !isLastCommentLine(out, line),
			"the repair left a dangling clause: %q", line)
	}
}

// isLastCommentLine reports whether a line is the final comment line of its run.
func isLastCommentLine(src, line string) bool {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		if l != line {
			continue
		}
		if i+1 >= len(lines) {
			return true
		}
		return !strings.HasPrefix(strings.TrimSpace(lines[i+1]), "//")
	}
	return false
}

// A block with no clean cut stays whole. Reporting a long comment is honest.
// Emitting a broken one is not.
func TestABlockWithNoCleanCutIsLeftAlone(t *testing.T) {
	body := "// One single unbroken sentence that runs on and on well past anything the " +
		"declaration beneath it can justify carrying, with no earlier full stop anywhere in it\n"
	src := "package p\n\n" + body + "const p = 1\n"

	require.NotEmpty(t, Check("x.go", src))
	out, changed := Fix("x.go", src)
	assert.False(t, changed, "a block with no clean cut must not be rewritten")
	assert.Equal(t, src, out)
}

// A block that does hold an earlier ending is still repaired, which is the
// control that proves the rule above did not simply stop cutting.
func TestABlockWithAnEarlierEndingIsStillRepaired(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// The bound every caller shares.",
		"// It was raised once, and the reason is that the old value truncated a",
		"// payload nobody had measured, which took a week of somebody's time.",
		"const p = 1",
	}, "\n")

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Contains(t, out, "The bound every caller shares.")
	assert.NotContains(t, out, "took a week")
	assert.Empty(t, Check("x.go", out))
}
