package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A license notice is never weighed: the edit gate refuses to cut one, so a
// finding on it could never be repaired.
func TestALicenseNoticeIsNotWeighed(t *testing.T) {
	src := "/*\n** Copyright 2015-2024 The Khronos Group Inc.\n**\n** SPDX-License-Identifier: Apache-2.0\n*/\nint x;\n"
	assert.Empty(t, CheckLength("x.h", src))
}

// A comment that is one parenthetical aside has no cut inside it that closes,
// so the cut drops the enclosing parenthesis.
func TestACommentThatIsOneAsideRepairs(t *testing.T) {
	src := "void f(void)\n{\n" +
		"   /* (ACO's isel setup re-runs the flag pass with after_lowering = true, which\n" +
		"    * is why unvectorized corpus entries matched RADV even before this ran here.) */\n" +
		"   nir_divergence_analysis(nir);\n}\n"
	require.NotEmpty(t, CheckLength("x.c", src))
	out, changed := FixLength("x.c", src)
	require.True(t, changed, "fix must repair every block it detects")
	assert.Empty(t, CheckLength("x.c", out), "nothing is left to report:\n%s", out)
	assert.Contains(t, out, "/* ACO's isel setup re-runs the flag pass", "the opening survives, unwrapped:\n%s", out)
}

// A /* */ comment over a short statement repairs like a line comment, and stays a block.
func TestABlockCommentRepairs(t *testing.T) {
	for name, src := range map[string]string{
		"aligned": "int f(void)\n{\n" +
			"   /* ACO uses explicit scratch args; on GFX9 the scratch offset is a\n" +
			"    * separate SGPR that the caller passes, and the wave offset follows it\n" +
			"    * in the order RADV declares them in its own argument setup code. */\n" +
			"   return 0;\n}\n",
		"plain": "int f(void)\n{\n" +
			"   /* ACO uses explicit scratch args; on GFX9 the scratch offset is a\n" +
			"      separate SGPR that the caller passes, and the wave offset follows it\n" +
			"      in the order RADV declares them in its own argument setup code. */\n" +
			"   return 0;\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, CheckLength("x.c", src), "the fixture must be a finding")
			out, changed := FixLength("x.c", src)
			require.True(t, changed, "fix must repair every block it detects")
			assert.Empty(t, CheckLength("x.c", out), "nothing is left to report:\n%s", out)
			assert.Contains(t, out, "/* ACO uses explicit scratch args", "the opening survives")
			assert.Equal(t, 1, strings.Count(out, "*/"), "the comment stays a single closed block:\n%s", out)
			assert.Contains(t, out, "   return 0;\n}", "the code is untouched")
		})
	}
}
