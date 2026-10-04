package commentfix

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
)

// Whatever the repair writes, it writes words. A rewrite that runs of the
// author's words into a single says something the author did not, and nobody
// reads the diff a hook applied.
func TestTheRepairNeverWeldsTwoWordsTogether(t *testing.T) {
	for _, in := range []string{
		"Batch GET two of them.",
		"It fails when this one does not carry it.",
		"The walk is dropping the one top directory.",
		"It reads from the first .gitmodules parser above.",
		"It reads the first .gitmodules above.",
		"The tree keeps the first entry left.",
		"It holds one of the two .gitmodules files.",
	} {
		t.Run(in, func(t *testing.T) {
			assert.Empty(t, welded(in, Reword(in)))
		})
	}
}

// welded names an output word made of the input carried side by side, and is
// empty when the repair kept them apart.
func welded(in, out string) string {
	have := set.Of(strings.Fields(in)...)
	for _, word := range strings.Fields(out) {
		if have.Contains(word) {
			continue
		}
		for cut := 1; cut < len(word); cut++ {
			if have.Contains(word[:cut]) && have.Contains(word[cut:]) {
				return word
			}
		}
	}
	return ""
}

// A quotation wrapped across comment lines is still a quotation. The check
// reads it the way the repair does, so the numbers a tool printed stay as the
// tool printed them.
func TestAQuotationWrappedAcrossLinesIsQuoted(t *testing.T) {
	src := "// schedsim's fit on the 28-hour log: \"prefill 10.0 ms + 65.874 us/token +\n" +
		"// 7.43e-11/token/ctx + 5.54e-16/token/ctx^2; decode 13.269 ms + 1.5 ms/req +\n" +
		"// 6e-09 s/token-ctx (accept 2.78); pool 1398667 tokens\".\n" +
		"const LOG_CALIBRATION = 1;\n"
	assert.Empty(t, Check("sim.ts", src))
	out := Fix("sim.ts", src)
	assert.Empty(t, Check("sim.ts", out.Text))
	assert.Equal(t, src, out.Text)
}

// A number the repair deletes where it sits goes whole. Deleting the digits of
// a decimal leaves its point and fraction standing as words nobody wrote.
func TestTheRepairNeverLeavesAFragmentOfANumber(t *testing.T) {
	src := "/*\n * Costs:\n    prefill 10.0 ms + 65.874 us/token, decode 1,024 us\n */\nconst x = 1;\n"
	out := Fix("sim.ts", src)
	require.True(t, out.Changed)
	assert.Empty(t, Check("sim.ts", out.Text))
	assert.Equal(t, "/*\n * Costs:\n    prefill ms + us/token, decode us\n */\nconst x = 1;\n", out.Text)
	fragment := regexp.MustCompile(`[0-9]|[A-Za-z(+^]\.|\s[.,]`)
	line := strings.Split(out.Text, "\n")[2]
	assert.False(t, fragment.MatchString(line), "fragment left in %q", line)
}

// The repair leaves a number behind for the sentence cut, and the cut is what
// clears the finding. Neither may leave the line saying nothing at all.
func TestTheRepairLeavesNoNumberStanding(t *testing.T) {
	src := "package p\n\n// It holds the two .gitmodules files.\nvar x int\n"
	out := Fix("x.go", src)
	require.True(t, out.Changed)
	assert.Empty(t, Check("x.go", out.Text))
}
