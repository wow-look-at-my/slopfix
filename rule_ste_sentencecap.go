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
			// go-toolchain docs/CI.md: a stray backtick closes a path with no opener, and the code spans after it stay whole.
			{Name: ste.IDSentenceCap + " with a stray backtick", Path: "docs/CI.md", Text: "Their jobs run .github/dats-fixtures/cache-profile.dats` asserts the tripwires themselves, run by the dats action in `host-build` the same way `same.dats` and `smoke.dats`.\n"},
			// sglang .github/scripts/ghcr-image-retention.ts: "an image a week old" is the subject after the colon, so no other subject is put in front of it.
			{Name: ste.IDSentenceCap + " with a measure after the subject", Path: "x.md", Text: "Ranked over every default-branch image, so the retention count is what bounds the history: an image a week old does not hold a slot that then falls to the newest of the images the age guard has already spared.\n"},
			// go-ipc .github/workflows/ci.yml: "the C and Python ones" is one noun phrase, so no division lands inside it.
			{Name: ste.IDSentenceCap + " around a gapped clause", Path: "x.md", Text: "Linux builds and tests the C, C++ and Python implementations and their peers first, macOS the C and Python ones, and Windows runs the interop matrix with the Go peer alone.\n"},
			// "and by B to Q" names a second actor for the participle.
			{Name: ste.IDSentenceCap + " with a second actor", Path: "x.md", Text: "Shared stage used by Dockerfile.worker to pre-install the static code lint extension and by the job scheduler (below) to seed the private build cache that it serves.\n"},
			// A dash aside before "so".
			{Name: ste.IDSentenceCap + " with a dash aside before so", Path: "x.md", Text: "A stale index is a different fault than a stale cache -- it is not on disk at all -- so it has to be rebuilt even when every file is on the server.\n"},
			// ", each with" gives each item what follows.
			{Name: ste.IDSentenceCap + " before each with", Path: "x.md", Text: "Lists what the client asked for and was denied, each with a one-step way to grant it going forward instead of searching through the access table above.\n"},
			// A bare verb after the comma shares the subject and its modal, so both are named again.
			{Name: ste.IDSentenceCap + " before a verb that shares the modal", Path: "x.md", Text: "A feature added to the registry after that rule was written must default to blocked, refuse the open default, and stay that way for every running job.\n"},
			// A heading line stands apart from the prose around it, and is judged for its own length.
			{Name: ste.IDSentenceCap + " in a comment heading", Path: "main.go", Text: "package main\n\n" +
				"// # This heading carries far more words than any reader can hold in mind at one time and it keeps going well past the cap of the rule today.\n" +
				"func main() {\n" +
				"\tprintln(\"first\")\n\tprintln(\"second\")\n\tprintln(\"third\")\n\tprintln(\"fourth\")\n\tprintln(\"fifth\")\n" +
				"\tprintln(\"sixth\")\n\tprintln(\"seventh\")\n\tprintln(\"eighth\")\n\tprintln(\"ninth\")\n\tprintln(\"tenth\")\n}\n"},
			// A "that" clause after a list is another item of it.
			{Name: ste.IDSentenceCap + " before a that clause in a list", Path: "x.md", Text: "- `queue_order_test.go` covers the priority order, the two-pass rule, the head-only window, case-folding, the size limit, that a failed fetch keeps the previous value.\n"},
			// sglang .github/workflows/fork-ci.yml: a comment in a run: script is prose, and divides like any comment.
			workflowCase("sentence-length in a run: script", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: |\n"+
				"          ON_DEFAULT=false\n\n"+
				"          # The image installs only this fork's kernel build, so a PyPI wheel of\n"+
				"          # the same version does not count: it lacks the fork's ops and archs.\n"+
				"          KERNEL_AVAILABLE=false\n"),
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
