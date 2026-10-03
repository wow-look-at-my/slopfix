package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// A banner of asterisks holds no prose, so the trim keeps nothing. The repair
// deletes the banner the way it deletes an empty line comment.
func TestABannerWithNoProseRepairsWithoutAPanic(t *testing.T) {
	src := "/***********************************************************************\n" +
		" *\n" +
		" */\n" +
		"void i915_init_state_functions(struct i915_context *i915);\n"
	require.NotEmpty(t, CheckLength("x.h", src), "the fixture must be a finding")
	out, changed := FixLength("x.h", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("x.h", out), out)
	assert.Contains(t, out, "void i915_init_state_functions(struct i915_context *i915);")
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
