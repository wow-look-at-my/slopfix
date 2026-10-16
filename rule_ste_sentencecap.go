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
			{Name: "a carrier never opens on a conjunction", Path: "governor.cpp", Text: carrierConjunctionCase, Want: carrierConjunctionWant},
			{Name: "a capital never renames a variable", Path: "governor.cpp", Text: variableOpensCase, Want: variableOpensWant},
			{Name: "a rule around comment prose stays at the edges", Path: "governor.cpp", Text: ruledCommentCase, Want: ruledCommentWant},
			{Name: "the objects of a preposition stay joined", Path: "CMakeLists.txt", Text: coordinatedObjectsCase, Want: coordinatedObjectsWant},
			{Name: "a clause of manner stays with its sentence", Path: "ident.go", Text: mannerClauseCase, Want: mannerClauseWant},
			{Name: "a list with no verb divides behind a carrier", Path: "moments.go", Text: verblessListCase, Want: verblessListWant},
		},
	})
}

// mannerClauseCase holds a sentence over the cap that ends on a clause of
// manner. "As a variable such as q is" stands as no sentence, so the division
// lands at ", or", where the clause after it stands.
const mannerClauseCase = `package ident

import "strings"

// lowerIdentifier reports a word that reads as a name in code. It carries a capital, a digit, an underscore or a dot inside it, or it is a single letter other than the article, as a variable such as q is.
func lowerIdentifier(word string) bool {
	if word == "" {
		return false
	}
	if len(word) == 1 {
		return word != "a"
	}
	for _, r := range word[1:] {
		if r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return strings.ToLower(word) != word
}
`

const mannerClauseWant = `package ident

import "strings"

// lowerIdentifier reports a word that reads as a name in code. It carries a capital, a digit, an underscore or a dot inside it. Otherwise, it is a single letter other than the article, as a variable such
// as q is.
func lowerIdentifier(word string) bool {
	if word == "" {
		return false
	}
	if len(word) == 1 {
		return word != "a"
	}
	for _, r := range word[1:] {
		if r == '_' || r == '.' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return strings.ToLower(word) != word
}
`

// verblessListCase holds a list over the cap that names terms and holds no
// verb, as the terms of a formula do. It closes at the last comma under the
// cap, and the rest stands behind "This also covers".
const verblessListCase = `package moments

// Per octant, total mass, mean position, second moment, weighted color, merged opacity, luma variance per cube, and flatness ratio of the stack against an opaque layer of the same cube.
func octantMoments(masses, positions, colors []float64) (mass, mean, color float64) {
	for i, m := range masses {
		mass += m
		mean += m * positions[i]
		color += m * colors[i]
	}
	if mass > 0 {
		mean /= mass
		color /= mass
	}
	return mass, mean, color
}
`

const verblessListWant = `package moments

// Per octant, total mass, mean position, second moment, weighted color, merged opacity, luma variance per cube. This also covers flatness ratio of the stack against an opaque layer of
// the same cube.
func octantMoments(masses, positions, colors []float64) (mass, mean, color float64) {
	for i, m := range masses {
		mass += m
		mean += m * positions[i]
		color += m * colors[i]
	}
	if mass > 0 {
		mean /= mass
		color /= mass
	}
	return mass, mean, color
}
`

// carrierConjunctionCase holds a noun phrase over the cap whose list after a
// colon ends on ", and". A carrier in front of "and the scalars it set" reads
// as no sentence, so the division lands at the colon.
const carrierConjunctionCase = `struct Governor {
  double lodBias_ = 1.0;
  // Pair-demand elasticity per rung, and the act whose demand response
  // measures one of them: the demand and scalars before the act, and the
  // scalars it set.
  struct {
    double eLod = kGovDemandElasticityPrior;
    double eRes = kGovDemandElasticityPrior;
    bool armed = false;
    double demand = 0, lod = 1, res = 1;
    double actLod = 1, actRes = 1;
  } probe_;
};
`

const carrierConjunctionWant = `struct Governor {
  double lodBias_ = 1.0;
  // Pair-demand elasticity per rung, and the act whose demand response
  // measures one of them. This is the demand and scalars before the act,
  // and the scalars it set.
  struct {
    double eLod = kGovDemandElasticityPrior;
    double eRes = kGovDemandElasticityPrior;
    bool armed = false;
    double demand = 0, lod = 1, res = 1;
    double actLod = 1, actRes = 1;
  } probe_;
};
`

// variableOpensCase holds a sentence over the cap whose words after a colon
// open on the variable q. A capital renames it, so "q" stays in lower case
// behind a carrier.
const variableOpensCase = `void applyQuality(double q);

double run(double lodNow, double resNow, double eLod, double eRes, bool lodOn, bool resPinned) {
  // Pair demand at quality q relative to now: q split over the free rungs
  // the way applyQuality spends it (before the res lattice), each rung's
  // ratio raised to its measured elasticity. Non-decreasing in q.
  const auto demandRatio = [&](double q) -> double {
    double lod = lodNow;
    double res = resNow;
    if (lodOn && !resPinned) {
      lod = std::clamp(q, kGovLodBiasMin, 1.0);
      res = std::clamp(q / lod, kGovResScaleMin, 1.0);
    } else if (lodOn) {
      lod = std::clamp(q / std::max(resNow, 0.01), kGovLodBiasMin, 1.0);
    } else if (!resPinned) {
      res = std::clamp(q / lodNow, kGovResScaleMin, 1.0);
    }
    return std::pow(lod / lodNow, eLod) * std::pow(res / resNow, eRes);
  };
  return demandRatio(1.0);
}
`

const variableOpensWant = `void applyQuality(double q);

double run(double lodNow, double resNow, double eLod, double eRes, bool lodOn, bool resPinned) {
  // Pair demand at quality q relative to now. This is q split over the
  // free rungs the way applyQuality spends it (before the res lattice),
  // each rung's ratio raised to its measured elasticity. Non-decreasing
  // in q.
  const auto demandRatio = [&](double q) -> double {
    double lod = lodNow;
    double res = resNow;
    if (lodOn && !resPinned) {
      lod = std::clamp(q, kGovLodBiasMin, 1.0);
      res = std::clamp(q / lod, kGovResScaleMin, 1.0);
    } else if (lodOn) {
      lod = std::clamp(q / std::max(resNow, 0.01), kGovLodBiasMin, 1.0);
    } else if (!resPinned) {
      res = std::clamp(q / lodNow, kGovResScaleMin, 1.0);
    }
    return std::pow(lod / lodNow, eLod) * std::pow(res / resNow, eRes);
  };
  return demandRatio(1.0);
}
`

// ruledCommentCase draws a rule of dashes at the start of each comment line
// and at the end of the last. The rule frames the prose, so the division
// rewraps the words between the rules and keeps every rule where it was.
const ruledCommentCase = `double sample(double demand) {
  // ---- elasticity probe: the first sample past the last act's cooldown is
  // ---- rendered at that act's scalars; its demand against the pre-act
  // ---- demand measures the elasticity of the rung the act moved -------
  if (probe_.armed && cooldownFrames_ == 0) {
    probe_.armed = false;
    if (lodBias_ == probe_.actLod && resScale_ == probe_.actRes) {
      const double lr = std::log(lodBias_ / probe_.lod);
      const double rr = std::log(resScale_ / probe_.res);
      const double dr = std::log(demand / probe_.demand);
      if (rr == 0.0 && std::abs(lr) >= minStep) {
        eLod_ = std::clamp(dr / lr, 0.0, kGovDemandElasticityPrior);
      } else if (lr == 0.0 && std::abs(rr) >= minStep) {
        eRes_ = std::clamp(dr / rr, 0.0, kGovDemandElasticityPrior);
      }
    }
  }
  return demand;
}
`

const ruledCommentWant = `double sample(double demand) {
  // ---- elasticity probe: the first sample past the last act's cooldown is
  // ---- rendered at that act's scalars. Its demand against the pre-act
  // ---- demand measures the elasticity of the rung the act moved -------
  if (probe_.armed && cooldownFrames_ == 0) {
    probe_.armed = false;
    if (lodBias_ == probe_.actLod && resScale_ == probe_.actRes) {
      const double lr = std::log(lodBias_ / probe_.lod);
      const double rr = std::log(resScale_ / probe_.res);
      const double dr = std::log(demand / probe_.demand);
      if (rr == 0.0 && std::abs(lr) >= minStep) {
        eLod_ = std::clamp(dr / lr, 0.0, kGovDemandElasticityPrior);
      } else if (lr == 0.0 && std::abs(rr) >= minStep) {
        eRes_ = std::clamp(dr / rr, 0.0, kGovDemandElasticityPrior);
      }
    }
  }
  return demand;
}
`

// coordinatedObjectsCase holds a CMake comment whose list after a colon joins
// clauses with semicolons. "on the copy and the in-place row wires" names a
// couple of objects of "on", so no division lands at its "and". Each item that
// is a clause of its own opens a sentence as it stands.
const coordinatedObjectsCase = `# animation-prepack-equivalence - the animation pre-pack's byte gate: every
# adopted frame of a synthetic clip lands records, cubeBuf, C and a rendered
# image equal to the live async tick wire's, on the copy and the in-place
# (UMA direct) row wires; a repeated C reuses its pipeline triple; an edge
# repack of an adopted frame and a resumed live tick match too. The parking
# leg packs a real multi-round chain at the default chunk clamp.
add_test(NAME animation-prepack-equivalence
  COMMAND $<TARGET_FILE:splat-vulkan> ${PARKING} --max-splats 5000 --fgen-test prepack
          --cube-edge 1 --validate)
`

const coordinatedObjectsWant = `# animation-prepack-equivalence - the animation pre-pack's byte gate. Every
# adopted frame of a synthetic clip lands records and cubeBuf. It also lands
# C and a rendered image equal to the live async tick wire's, on the copy
# and the in-place (UMA direct) row wires. A repeated C reuses its pipeline
# triple. An edge repack of an adopted frame and a resumed live tick match
# too. The parking leg packs a real multi-round chain at the default chunk
# clamp.
add_test(NAME animation-prepack-equivalence
  COMMAND $<TARGET_FILE:splat-vulkan> ${PARKING} --max-splats 5000 --fgen-test prepack
          --cube-edge 1 --validate)
`

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
