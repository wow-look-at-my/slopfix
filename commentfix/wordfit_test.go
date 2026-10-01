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
	assert.Contains(t, out, "(VGPR, SGPR, LDS, hardware cap) and names the constraint.\n", "the cut lands before the phrase the budget cannot hold:\n%s", out)
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

// A clause cut keeps a finite verb. Without one it keeps a fragment.
func TestAClauseCutKeepsAVerb(t *testing.T) {
	src := "package p\n\nfunc f() {\n" +
		"\t// A label-only line, plus a same-line label, both attach to the following\n" +
		"\t// instruction so a branch target resolves to an instruction index.\n" +
		"\tinsts, err := parse(src)\n\tuse(insts, err)\n}\n"
	out, changed := FixLength("p.go", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("p.go", out))
	assert.Contains(t, out, "both attach to the following instruction", "the cut keeps the verb:\n%s", out)
}

// The budget counts characters, so a cut that fits on one long line is kept.
// Wrapped at the column cap, this kept only "RegistersFromCompiler is true."
func TestACutMayUseOneLongLine(t *testing.T) {
	src := "package p\n\ntype R struct {\n" +
		"\t// RegistersFromCompiler is true when MaxSGPR/MaxVGPR/SpilledSGPRs/\n" +
		"\t// SpilledVGPRs/Occupancy are the compiler-authoritative figures ACO reported\n" +
		"\t// for a real compile (via SetCompiledRegisters), and false when they are\n" +
		"\t// re-derived from the disassembly.\n" +
		"\tRegistersFromCompiler bool\n}\n"
	out, changed := FixLength("p.go", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("p.go", out))
	assert.Contains(t, out, "are the compiler-authoritative figures", "the cut keeps the claim:\n%s", out)
}

// A clause mark near the opening keeps less than a word cut near the budget.
func TestTheCutThatKeepsMoreWins(t *testing.T) {
	src := "package p\n\ntype C struct {\n" +
		"\t// Progress, when set, is called by the sampler every progressTick with the\n" +
		"\t// current series' sample count and confidence interval. RunAll installs a\n" +
		"\t// live-line printer; tests leave it nil.\n" +
		"\tProgress func(samples int, rmePct, targetRMEPct float64)\n}\n"
	out, changed := FixLength("p.go", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("p.go", out))
	assert.Contains(t, out, "is called by the sampler every progressTick", "the longer cut wins:\n%s", out)
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

func TestFixRepairsASentenceNoClauseCutFits(t *testing.T) {
	require.NotEmpty(t, CheckLength("p.go", unboundedSentence), "the fixture must be a finding")
	out, changed := FixLength("p.go", unboundedSentence)
	require.True(t, changed, "fix must repair every block it detects")
	assert.Empty(t, CheckLength("p.go", out), "the repair must leave nothing for the check to report:\n%s", out)
	assert.Contains(t, out, "// cacheKeyFor returns the stable composite lookup key", "the opening words survive")
	assert.Contains(t, out, "func cacheKeyFor() int { return 0 }", "the code is untouched")
}
