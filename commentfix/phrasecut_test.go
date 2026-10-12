package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A single sentence with no clause boundary and no tail opener, over a
// one-line function. No cut leaves a whole sentence.
const unboundedSentence = "package p\n\n" +
	"// cacheKeyFor returns the stable composite lookup key built from the resolved descriptor\n" +
	"// set index plus binding slot plus array element offset plus sampler identity hash value.\n" +
	"func cacheKeyFor() int { return 0 }\n"

// fitsOrStays asserts both outcomes a length repair may have. The block fits
// and keeps want, or no cut fits, the block stays as written, and no finding
// names it.
func fitsOrStays(t *testing.T, src, want string) {
	t.Helper()
	hits := CheckLength("p.go", src)
	out, _ := FixLength("p.go", src)
	if len(hits) == 0 {
		assert.Equal(t, src, out, "a block no cut fits keeps its prose")
		return
	}
	assert.Empty(t, CheckLength("p.go", out))
	assert.Contains(t, out, want, "the cut keeps the claim:\n%s", out)
}

// A cut before an aside keeps the claim the aside explains. A cut before
// "that binds" changes which constraint the comment names, so it never lands there.
func TestACutLandsBeforeAnAside(t *testing.T) {
	src := "package p\n\ntype T struct {\n" +
		"\t// OccupancyBounds breaks Occupancy into its per-resource wave/SIMD bounds\n" +
		"\t// (VGPR, SGPR, LDS, hardware cap) and names the constraint that binds.\n" +
		"\tOccupancyBounds *OccupancyBounds\n}\n"
	out, changed := FixLength("p.go", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("p.go", out))
	assert.Contains(t, out, "// OccupancyBounds breaks Occupancy into its per-resource wave/SIMD bounds.\n", out)
	assert.NotContains(t, out, "names the constraint.", out)

	src = "package p\n\nfunc f() {\n" +
		"\t// The same stream with a >wave workgroup is NOT the lane-index idiom (the\n" +
		"\t// forward compiler will have gone through tg_size instead).\n" +
		"\tcheck()\n}\n"
	out, changed = FixLength("p.go", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("p.go", out))
	assert.Contains(t, out, "is NOT the lane-index idiom.\n", "the claim survives:\n%s", out)
}

// A cut before "so" drops the purpose and keeps the claim with its verb.
func TestACutBeforeSoKeepsTheClaim(t *testing.T) {
	src := "package p\n\nfunc f() {\n" +
		"\t// A label-only line, plus a same-line label, both attach to the following\n" +
		"\t// instruction so a branch target resolves to an instruction index.\n" +
		"\tinsts, err := parse(src)\n\tuse(insts, err)\n}\n"
	out, changed := FixLength("p.go", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("p.go", out))
	assert.Contains(t, out, "both attach to the following instruction.\n", "the cut keeps the verb:\n%s", out)
}

// The budget counts characters, so a cut that fits on one long line is kept.
func TestACutMayUseOneLongLine(t *testing.T) {
	src := "package p\n\ntype R struct {\n" +
		"\t// RegistersFromCompiler is true when MaxSGPR/MaxVGPR/SpilledSGPRs/\n" +
		"\t// SpilledVGPRs/Occupancy are the compiler-authoritative figures ACO reported\n" +
		"\t// for a real compile (via SetCompiledRegisters), and false when they are\n" +
		"\t// re-derived from the disassembly.\n" +
		"\tRegistersFromCompiler bool\n}\n"
	fitsOrStays(t, src, "are the compiler-authoritative figures")
}

// The cut never stops between words, so a sentence no boundary divides either
// fits or stays for a rewrite by hand.
func TestNoCutStopsBetweenWords(t *testing.T) {
	src := "package p\n\ntype C struct {\n" +
		"\t// Progress, when set, is called by the sampler every progressTick with the\n" +
		"\t// current series' sample count and confidence interval. RunAll installs a\n" +
		"\t// live-line printer; tests leave it nil.\n" +
		"\tProgress func(samples int, rmePct, targetRMEPct float64)\n}\n"
	fitsOrStays(t, src, "// Progress, when set, is called by the sampler")
}

// A comment over a literal element with a trailing comment is weighed against
// the element. Read as a comment line, the element measured as no code at all.
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

// A sentence with no clause boundary still has a cut: the phrase after the noun.
func TestASentenceWithNoClauseBoundaryCutsAtAPhrase(t *testing.T) {
	hits := CheckLength("p.go", unboundedSentence)
	require.NotEmpty(t, hits, "the fixture must be a finding")
	for _, hit := range hits {
		assert.True(t, hit.Repairable)
	}
	out, _ := FixLength("p.go", unboundedSentence)
	assert.Equal(t, "package p\n\n// cacheKeyFor returns the stable composite lookup key.\nfunc cacheKeyFor() int { return 0 }\n", out, "the participle phrase after the noun goes, and the clause before it stands")
	assert.Empty(t, CheckLength("p.go", out))
}
