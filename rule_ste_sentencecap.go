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
			{Name: "a carrier never opens on a conjunction", Path: "governor.cpp", Text: carrierConjunctionCase, Want: carrierConjunctionWant},
			{Name: "a capital never renames a variable", Path: "governor.cpp", Text: variableOpensCase, Want: variableOpensWant},
			{Name: "a rule around comment prose stays at the edges", Path: "governor.cpp", Text: ruledCommentCase, Want: ruledCommentWant},
			{Name: "the objects of a preposition stay joined", Path: "CMakeLists.txt", Text: coordinatedObjectsCase, Want: coordinatedObjectsWant},
		},
	})
}

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
# adopted frame of a synthetic clip lands records and cubeBuf. Every adopted
# frame also lands C and a rendered image equal to the live async tick
# wire's. That frame is on the copy and the in-place (UMA direct) row wires.
# A repeated C reuses its pipeline triple. An edge repack of an adopted
# frame and a resumed live tick match too. The parking leg packs a real
# multi-round chain at the default chunk clamp.
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
