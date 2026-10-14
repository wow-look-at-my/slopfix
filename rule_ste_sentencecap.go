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
				Name: "a clause whose long tail has no verb-safe cut", Path: "tail.md",
				Text: "The gate reads the cache file of the build host for each module of the tree in the order of the paths of the list of the repository at the start of the run.\n",
				Want: "The gate reads the cache file of the build host for each module of the tree. This holds in the order of the paths of the list of the repository at the start of the run.\n",
			},
			{
				Name: "a sentence with no verb at all", Path: "noverb.md",
				Text: "A list of the cache files of the build host for each module of the tree in the order of the paths of the list of the repository at the start of the run.\n",
				Want: "This covers a list of the cache files of the build host for each module of the tree. This holds in the order of the paths of the list of the repository at the start of the run.\n",
			},
			{
				// From splat2's system-lock.h: the division kept "at a time" off its verb.
				Name: "a so clause keeps its adverbial", Path: "lock.md",
				Text: "main() holds the \"gpu\" lock for every splat-vulkan run except --help and --dump-layouts, so only one splat-vulkan uses the GPU at a time.\n",
				Want: "main() holds the \"gpu\" lock for every splat-vulkan run except --help and --dump-layouts. Only one splat-vulkan uses the GPU at a time.\n",
			},
			{
				// From splat2's system-lock.h: the division after the colon dropped the verb "takes a ticket in".
				Name: "a clause after a colon keeps its own subject", Path: "queue.md",
				Text: "Every request takes a ticket in \"<lock file>.queue/\": a sequence number from the counter file, taken under flock on that file, names a ticket file that its owner holds flock(LOCK_EX) on for as long as it waits or holds.\n",
				Want: "Every request takes a ticket in \"<lock file>.queue/\". A sequence number from the counter file, taken under flock on that file, names a ticket file. Its owner holds flock(LOCK_EX) on that file for as long as it waits or holds.\n",
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
