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
				Text: "- Measured fact: each static packs into the `8.33 ms` render windows at about `66%` duty, so one static spans about `4` windows (`33 ms`) against the `27.8 ms` frame tick.\n",
				Want: "- Measured fact: each static packs into the `8.33 ms` render windows at about `66%` duty. One static spans about `4` windows (`33 ms`) against the `27.8 ms` frame tick.\n",
			},
			{
				Name: "a comment sentence behind a dash and a parenthesis", Path: "path.h",
				Text: "class SlotPool {\n" +
					"  // The ONE slot-capacity law (pre-size envelope + adaptive high-water +\n" +
					"  // budget/device clamps) \u2014 reserve() and growSlots() MUST agree byte-for-\n" +
					"  // byte or growth would thrash through reserve()'s slotsChanged check.\n" +
					"  // Updates growthCap_/coarseGrowthCap_ as a side effect.\n" +
					"  void computeSlotCapacities(const Context& ctx, uint32_t F, uint32_t Fc,\n" +
					"                             uint32_t& slotCapacity,\n" +
					"                             uint32_t& coarseSlotCapacity);\n" +
					"};\n",
				Want: "class SlotPool {\n" +
					"  // The ONE slot-capacity law (pre-size envelope + adaptive high-water +\n" +
					"  // budget/device clamps) \u2014 reserve() and growSlots() MUST agree\n" +
					"  // byte-for-byte. Otherwise, growth would thrash through reserve()'s\n" +
					"  // slotsChanged check. Updates growthCap_/coarseGrowthCap_ as a side\n" +
					"  // effect.\n" +
					"  void computeSlotCapacities(const Context& ctx, uint32_t F, uint32_t Fc,\n" +
					"                             uint32_t& slotCapacity,\n" +
					"                             uint32_t& coarseSlotCapacity);\n" +
					"};\n",
				Files: map[string]string{"path.cpp": "int growSlots();\nint slotsChanged;\nint growthCap_;\nint coarseGrowthCap_;\nint reserve();\n"},
			},
			{
				Name: "a clause whose long tail has no verb-safe cut", Path: "tail.md",
				Text: "The gate reads the cache file of the build host for each module of the tree in the order of the paths of the list of the repository at the start of the run.\n",
				Want: "The gate reads the cache file of the build host for each module of the tree. That tree is in the order of the paths of the list of the repository at the start of the run.\n",
			},
			{
				Name: "a formula is data that no division enters", Path: "msg.txt",
				Text: "The gate fits the samples with k1 = f/3 + 4f^2 - 5f^3/2 for every frame of the run and it writes the weight into the table that the loader reads at startup.\n",
				Want: "The gate fits the samples with k1 = f/3 + 4f^2 - 5f^3/2 for every frame of the run. It writes the weight into the table that the loader reads at startup.\n",
			},
			{
				Name: "a relative clause with no that stays on its noun", Path: "msg.txt",
				Text: "flushPage writes new index or journal entries into the segment no pending reader holds and flips to it before the next full commit starts its sync pass.\n",
				Want: "flushPage writes new index or journal entries into the segment no pending reader holds. flushPage flips to it before the next full commit starts its sync pass.\n",
			},
			{
				Name: "a sentence with no verb at all", Path: "noverb.md",
				Text: "A list of the cache files of the build host for each module of the tree in the order of the paths of the list of the repository at the start of the run.\n",
				Want: "A list of the cache files of the build host for each module of the tree. That tree is in the order of the paths of the list of the repository at the start of the run.\n",
			},
			{
				Name: "a so clause keeps its adverbial", Path: "lock.md",
				Text: "main() holds the \"disk\" lock for every sync-tool run except --help and --dump-config, so only one sync-tool uses the disk at a time.\n",
				Want: "main() holds the \"disk\" lock for every sync-tool run except --help and --dump-config. Only one sync-tool uses the disk at a time.\n",
			},
			{
				Name: "a clause after a colon keeps its own subject", Path: "queue.md",
				Text: "Every job takes a slot in \"<state dir>.pool/\": a serial number from the index file, taken under flock on that file, names a slot file that its owner holds flock(LOCK_EX) on for as long as it waits or holds.\n",
				Want: "Every job takes a slot in \"<state dir>.pool/\". A serial number from the index file, taken under flock on that file, names a slot file. That file is one that its owner holds flock(LOCK_EX) on for as long as it waits or holds.\n",
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
