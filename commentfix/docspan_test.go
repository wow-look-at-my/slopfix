package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Rust doc comment ends its node on the declaration it documents, so the block
// span carried that line.
func TestADocCommentSpanStopsAtTheComment(t *testing.T) {
	for name, src := range map[string]string{
		"outer doc": essay("///") + "const P: i32 = 1;\n",
		"inner doc": essay("//!") + "const P: i32 = 1;\n",
		"line":      essay("//") + "const P: i32 = 1;\n",
	} {
		for _, b := range blocks("x.rs", src) {
			for _, line := range b.text {
				assert.True(t, opensWithMarker(line) || strings.TrimSpace(line) == "",
					"%s: the block span carries code: %q", name, line)
			}
		}
	}
}

// Each doc comment ended its node on the next row, so the next comment after a
// single declaration read as adjoining. The run then held the declaration, and
// the repair deleted it.
func TestDocCommentsSplitByADeclarationAreSeparateBlocks(t *testing.T) {
	src := "impl P {\n" +
		"    /// Short.\n" +
		"    pub const A: u32 = 2;\n" +
		indent(essay("///"), "    ") +
		"    pub const B: u32 = 8;\n" +
		"    /// Short too.\n" +
		"    pub fn c(v: u32) -> u32 {\n" +
		"        v\n" +
		"    }\n" +
		"}\n"
	for _, b := range blocks("x.rs", src) {
		for _, line := range b.text {
			assert.True(t, opensWithMarker(line), "the block span carries code: %q", line)
		}
	}
	fixed, changed := FixLength("x.rs", src)
	require.True(t, changed, "the essay was not cut")
	for _, line := range strings.Split(src, "\n") {
		if strings.TrimSpace(line) == "" || opensWithMarker(line) {
			continue
		}
		assert.Contains(t, fixed, line, "the repair deleted code")
	}
	assert.Contains(t, fixed, "/// Short.")
	assert.Contains(t, fixed, "/// Short too.")
}

func indent(text, prefix string) string {
	lines := strings.SplitAfter(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "")
}

func TestTheRepairKeepsTheDeclarationUnderADocComment(t *testing.T) {
	for name, src := range map[string]string{
		"outer doc": essay("///") + "const P: i32 = 1;\n",
		"inner doc": essay("//!") + "const P: i32 = 1;\n",
		"line":      essay("//") + "const P: i32 = 1;\n",
	} {
		fixed, changed := FixLength("x.rs", src)
		require.True(t, changed, "%s: the repair did nothing", name)
		assert.Contains(t, fixed, "const P: i32 = 1;",
			"%s: the repair took the declaration with the prose", name)
	}
}
