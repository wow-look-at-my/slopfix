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
				assert.True(t, commentLine(line) || strings.TrimSpace(line) == "",
					"%s: the block span carries code: %q", name, line)
			}
		}
	}
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
