package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// ste/sentence-length: a sentence over the STE word cap. The repair divides it
// at a clause boundary, and between words only where no boundary reads.
func init() {
	RegisterRule(RuleSpec{
		ID:       ste.IDSentenceCap,
		Category: RuleSTE,
		Detect:   detectSentenceCap,
		Autofix:  autofixSentenceCap,
		Cases: []RuleCase{
			{Name: ste.IDSentenceCap, Path: "x.md", Text: "This sentence carries far more words than any reader can hold in mind at one time and it keeps going well past the cap of the rule today.\n"},
			{
				Name: "a list item whose code spans count as words divides at so", Path: "x.md",
				Text: "- Measured fact: each real packs into the `8.33 ms` present windows at about `66%` duty, so one real spans about `4` windows (`33 ms`) against the `27.8 ms` content tick.\n",
				Want: "- Measured fact: each real packs into the `8.33 ms` present windows at about `66%` duty. One real spans about `4` windows (`33 ms`) against the `27.8 ms` content tick.\n",
			},
			{
				// From splat2's froxel-path-members.h, where `slopfix fix` left the finding standing.
				Name: "a comment sentence behind a dash and a parenthesis", Path: "path.h",
				Text: "class FroxelPath {\n" +
					"  // The ONE pair-capacity law (pre-size envelope + adaptive high-water +\n" +
					"  // budget/device clamps) — prepare() and growPairs() MUST agree byte-for-\n" +
					"  // byte or growth would ping-pong through prepare()'s pairsChanged check.\n" +
					"  // Updates growthCap_/coarseGrowthCap_ as a side effect.\n" +
					"  void computePairCapacities(const Context& ctx, uint32_t F, uint32_t Fc,\n" +
					"                             uint32_t& pairCapacity,\n" +
					"                             uint32_t& coarsePairCapacity);\n" +
					"};\n",
				Want: "class FroxelPath {\n" +
					"  // The ONE pair-capacity law (pre-size envelope + adaptive high-water +\n" +
					"  // budget/device clamps) — prepare() and growPairs() MUST agree\n" +
					"  // byte-for-byte. Otherwise, growth would ping-pong through prepare()'s\n" +
					"  // pairsChanged check. Updates growthCap_/coarseGrowthCap_ as a side effect.\n" +
					"  void computePairCapacities(const Context& ctx, uint32_t F, uint32_t Fc,\n" +
					"                             uint32_t& pairCapacity,\n" +
					"                             uint32_t& coarsePairCapacity);\n" +
					"};\n",
				Files: map[string]string{"path.cpp": "int growPairs();\nint pairsChanged;\nint growthCap_;\nint coarseGrowthCap_;\nint prepare();\n"},
			},
			{
				Name: "an object after or takes the main verb again", Path: "x.md",
				Text: "The rule answers the number at offset at and the words up to the plural noun it counts, as in \"0 validation errors\", or \"\" when no plural noun ends a phrase it governs.\n",
				Want: "The rule answers the number at offset at and the words up to the plural noun it counts, as in \"0 validation errors\". Otherwise, the rule answers \"\" when no plural noun ends a phrase it governs.\n",
			},
		},
	})
}

// detectSentenceCap answers each sentence over the word cap that the case
// carries.
func detectSentenceCap(c RuleCase) []ste.Finding {
	return caseFindings(c, ste.IDSentenceCap)
}

// autofixSentenceCap divides the over-long sentence at a clause boundary, and
// answers the case without the finding.
func autofixSentenceCap(c RuleCase) RuleCase {
	return caseAutofix(c, ste.IDSentenceCap)
}
