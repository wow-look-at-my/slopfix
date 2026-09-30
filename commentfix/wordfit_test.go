package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A single sentence with no clause boundary, over a one-line function: no
// sentence cut and no clause cut fits, so only a word cut can repair it.
const unboundedSentence = "package p\n\n" +
	"// cacheKeyFor returns the stable composite lookup key built from the resolved descriptor\n" +
	"// set index plus binding slot plus array element offset plus sampler identity hash value.\n" +
	"func cacheKeyFor() int { return 0 }\n"

// A word cut ends a phrase. Cut mid-phrase, this read "... hardware cap) and names."
func TestAWordCutEndsAPhrase(t *testing.T) {
	src := "package p\n\ntype T struct {\n" +
		"\t// OccupancyBounds breaks Occupancy into its per-resource wave/SIMD bounds\n" +
		"\t// (VGPR, SGPR, LDS, hardware cap) and names the constraint that binds.\n" +
		"\tOccupancyBounds *OccupancyBounds\n}\n"
	require.NotEmpty(t, CheckLength("p.go", src))
	out, changed := FixLength("p.go", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("p.go", out))
	assert.Contains(t, out, "(VGPR, SGPR, LDS, hardware cap).\n", "the cut lands before the phrase the budget cannot hold:\n%s", out)
}

// A cut before an aside keeps the claim the aside only explains.
func TestAWordCutLandsBeforeAnAside(t *testing.T) {
	src := "package p\n\nfunc f() {\n" +
		"\t// The same stream with a >wave workgroup is NOT the lane-index idiom (the\n" +
		"\t// forward compiler will have gone through tg_size instead).\n" +
		"\tcheck()\n}\n"
	out, changed := FixLength("p.go", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("p.go", out))
	assert.Contains(t, out, "is NOT the lane-index idiom.\n", "the claim survives:\n%s", out)
}

func TestFixRepairsASentenceNoClauseCutFits(t *testing.T) {
	require.NotEmpty(t, CheckLength("p.go", unboundedSentence), "the fixture must be a finding")
	out, changed := FixLength("p.go", unboundedSentence)
	require.True(t, changed, "fix must repair every block it detects")
	assert.Empty(t, CheckLength("p.go", out), "the repair must leave nothing for the check to report:\n%s", out)
	assert.Contains(t, out, "// cacheKeyFor returns the stable composite lookup key", "the opening words survive")
	assert.Contains(t, out, "func cacheKeyFor() int { return 0 }", "the code is untouched")
}
