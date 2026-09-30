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

func TestFixRepairsASentenceNoClauseCutFits(t *testing.T) {
	require.NotEmpty(t, CheckLength("p.go", unboundedSentence), "the fixture must be a finding")
	out, changed := FixLength("p.go", unboundedSentence)
	require.True(t, changed, "fix must repair every block it detects")
	assert.Empty(t, CheckLength("p.go", out), "the repair must leave nothing for the check to report:\n%s", out)
	assert.Contains(t, out, "// cacheKeyFor returns the stable composite lookup key", "the opening words survive")
	assert.Contains(t, out, "func cacheKeyFor() int { return 0 }", "the code is untouched")
}
