package slopfix

import (
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// comments/number: a number in a comment is a count of what exists today. The
// repair says it in words, or names the item the number was counting.
func init() {
	RegisterRule(RuleSpec{
		ID:       commentfix.ID,
		Category: RuleComments,
		Detect:   detectCommentNumber,
		Autofix:  autofixCommentNumber,
		Cases: []RuleCase{
			{Name: commentfix.ID, Path: "main.go", Text: "package main\n\n// There are 3 modes today.\nfunc main() {}\n"},
			{
				Name: "a verb before a zero count is no label", Path: "gate.cmake",
				Text: "# Each run must exit 0, report the cache entries and the retry mode, report 0 validation errors and write its log.\nset(A 1)\n",
				Want: "# Each run must exit 0, report the cache entries and the retry mode, report no validation errors and write its log.\nset(A 1)\n",
			},
			{
				Name: "an exit status wrapped onto the next comment line", Path: "gate.h",
				Text: "// Under --strict the lint errors of the build phases fail the run (exit\n// 2), the convention of the old suite.\nint runSuite() {\n\tint failed = lint();\n\treturn failed;\n}\n",
				Want: "// Under --strict the lint errors of the build phases fail the run\n// (exit 2), the convention of the old suite.\nint runSuite() {\n\tint failed = lint();\n\treturn failed;\n}\n",
			},
			{
				Name: "a comment after code continues on the aligned lines under it", Path: "opts.h",
				Text: "struct Options {\n" +
					"  bool tracePrefix = true;    // --prefix-mode picks the prefix the trace uses.\n" +
					"  bool traceCounts = false;   // --trace-counts 0|1: compile the counters of\n" +
					"                              //   the trace in. The log bytes stay the same.\n" +
					"  double poolBudgetMb = 0.0;  // --pool-budget-mb caps the pool buffer.\n" +
					"};\n",
				Want: "struct Options {\n" +
					"  bool tracePrefix = true;    // --prefix-mode picks the prefix the trace uses.\n" +
					"  bool traceCounts = false;   // The log bytes stay the same.\n" +
					"  double poolBudgetMb = 0.0;  // --pool-budget-mb caps the pool buffer.\n" +
					"};\n",
			},
			{
				Name: "a rule line that frames a banner stays a line of its own", Path: "banner.h",
				Text: bannerOpen + "// The gate renders the scene in each mode and compares the bytes of the frames of every run against the baseline.\n// There are 3 modes today.\n" + bannerClose,
				Want: bannerOpen + "// The gate renders the scene in each mode and compares the bytes of the frames of every run against the baseline.\n// There are modes today.\n" + bannerClose,
			},
			{Name: "a cut keeps the rule that opens each line", Path: "test-governor.cpp", Text: ruledNumbersCase, Want: ruledNumbersWant},
			{Name: "index zero after a possessive is the first item", Path: "froxel-path.h", Text: chunkZeroCase, Want: chunkZeroWant},
			{Name: "index zero is the first item", Path: "froxel-path.cpp", Text: frameZeroCase, Want: frameZeroWant},
			{Name: "a value is cut with its sentence", Path: "warp-pass.h", Text: valuesCase, Want: valuesWant},
			{Name: "a cut paragraph takes its break with it", Path: "warp-pass.h", Text: cutParagraphCase, Want: cutParagraphWant},
			// A zero that is the subject of a verb, as in buildhost's upload.ts.
			{Name: commentfix.ID + "-zero-subject", Path: "size.ts", Text: "/** The size from the input: empty is the default, 0 disables chunking. */\nexport function size(input: string): number {\n\treturn input.length;\n}\n"},
		},
	})
}

// bannerRule is the row of "=" a banner comment opens and closes on.
const bannerRule = "// ============================================================================\n"

// bannerOpen and bannerClose frame a banner comment above a function long
// enough that the length rule leaves the comment whole.
const (
	bannerOpen  = bannerRule
	bannerClose = bannerRule + "int runGate() {\n\tint failed = 0;\n" +
		"\tfailed += compareColor();\n\tfailed += compareDepth();\n\tfailed += compareNormal();\n\tfailed += compareAlpha();\n" +
		"\tfailed += compareStencil();\n\tfailed += compareMotion();\n\tfailed += compareVelocity();\n\tfailed += compareCoverage();\n" +
		"\tfailed += compareHistory();\n\tfailed += compareExposure();\n\tfailed += compareBloom();\n\tfailed += compareTonemap();\n" +
		"\treturn failed;\n}\n"
)

// cutParagraphCase opens with a paragraph of a single sentence the number cut
// takes whole. The break under it then separates nothing, so it goes too.
const cutParagraphCase = `struct WarpPass {
  // The pass ships 4 hooks.
  //
  // It reads N px/texel (N in {1,2,4,8}; 0 = off).
  // Pin --warp-field 0 for the pre-field warp.
  void record(int n) {
    density_ = n;
    mask_ = 0;
    field_ = n * 2;
    cells_ = n + 1;
  }
};
`

const cutParagraphWant = `struct WarpPass {
  void record(int n) {
    density_ = n;
    mask_ = 0;
    field_ = n * 2;
    cells_ = n + 1;
  }
};
`

// chunkZeroCase names an item by its index: "chunk 0" is the first chunk, and
// "one chunk" says another thing.
const chunkZeroCase = `struct FroxelPath {
  Buffer srcRows_;
  // No host memcpy runs at all: the pack's chunk 0 copies straight from the
  // caller's buffer into srcRows_.
  bool beginFeedTickFromBuffer(Context& ctx, VkBuffer rows, uint32_t count, double edge,
                               std::string* why = nullptr, bool direct = false);
};
`

const chunkZeroWant = `struct FroxelPath {
  Buffer srcRows_;
  // No host memcpy runs at all: the pack's first chunk copies straight from
  // the caller's buffer into srcRows_.
  bool beginFeedTickFromBuffer(Context& ctx, VkBuffer rows, uint32_t count, double edge,
                               std::string* why = nullptr, bool direct = false);
};
`

// valuesCase holds a count beside the value of a flag, a set literal and an
// assignment. The repair says the count in words and cuts each sentence that
// holds a value.
const valuesCase = `struct WarpPass {
  // Run answers one call 3 ways and ships 4 hooks.
  //
  // It reads N px/texel (N in {1,2,4,8}; 0 = off).
  // Pin --warp-field 0 for the pre-field warp.
  void record(int n) {
    density_ = n;
    mask_ = 0;
    field_ = n * 2;
    cells_ = n + 1;
  }
};
`

const valuesWant = `struct WarpPass {
  // Run answers one call multiple ways and ships hooks.
  void record(int n) {
    density_ = n;
    mask_ = 0;
    field_ = n * 2;
    cells_ = n + 1;
  }
};
`

// frameZeroCase names frame zero at the start of a sentence and before a
// possessive ending.
const frameZeroCase = `void FroxelPath::finishPrepack(Context& ctx) {
  // Frame 0 goes live through the ordinary adopt, which installs frame 0's
  // triple from the map.
  if (prepack_.empty()) {
    livePrepackFrame_ = kNoPrepackFrame;
    prepackPending_ = 0;
    adoptPrepacked(ctx);
  }
}
`

const frameZeroWant = `void FroxelPath::finishPrepack(Context& ctx) {
  // The first frame goes live through the ordinary adopt, which installs
  // the first frame's triple from the map.
  if (prepack_.empty()) {
    livePrepackFrame_ = kNoPrepackFrame;
    prepackPending_ = 0;
    adoptPrepacked(ctx);
  }
}
`

// ruledNumbersCase opens every comment line with a rule of dashes. The cut
// takes the sentences that state numbers, and the rewrap puts the rule back
// at the start of each line, never between words.
const ruledNumbersCase = `void testInertLodRung() {
  // ---- (l) INERT LOD RUNG: the M5 animation telemetry (1.23M splats per
  // ---- frame, 1280x720 internal, fgen feed budget 1000/36 - 4 = 23.8 ms).
  // ---- Measured headless: lod-bias 1 -> 0.42 moved pair demand by -0.2%,
  // ---- and res 1 -> 0.5 moved it 3.41M -> 2.60M (elasticity 0.39). Raster
  // ---- 50.0 ms at res 1 and 19.4 ms at res 0.5 give kPx 39.8 ms/Mpx and
  // ---- kPairR 3.94 ns/pair; bin+scatter+sort 3.8 ms gives kBss 1.12. Only
  // ---- the res floor lands inside the deadband. The governor must learn
  // ---- both elasticities from its own acts and reach that floor without
  // ---- walking the inert lod rung act by act.
  {
    Governor gov;
    Governor::Config cfg;
    cfg.budgetMs = 1000.0 / 36.0 - 4.0;
    cfg.lodAvailable = true;
    World w;
    w.demand0 = 3.41e6;
    w.kPx = 39.8;
    w.kPairR = 3.94;
    w.kBss = 1.12;
    w.fixed = 1.0;
    w.canvasW = 1280;
    w.canvasH = 720;
    w.eLod = 0.0;
    w.eRes = 0.39;
    int floorSample = -1;
    int lodOnlyActs = 0;
    int acts = 0;
    for (int i = 0; i < 200; i++) {
      Governor::Sample s = w.step(gov);
      const double lodBefore = gov.lodBias();
      const double resBefore = gov.resolutionScale();
      if (gov.sample(cfg, s)) {
        acts++;
        if (gov.resolutionScale() == resBefore && gov.lodBias() != lodBefore) lodOnlyActs++;
      }
      if (floorSample < 0 && gov.resolutionScale() == kGovResScaleMin) floorSample = i;
    }
  }
}
`

const ruledNumbersWant = `void testInertLodRung() {
  // ---- Only the res floor lands inside the deadband. The governor must
  // ---- learn both elasticities from its own acts and reach that floor
  // ---- without walking the inert lod rung act by act.
  {
    Governor gov;
    Governor::Config cfg;
    cfg.budgetMs = 1000.0 / 36.0 - 4.0;
    cfg.lodAvailable = true;
    World w;
    w.demand0 = 3.41e6;
    w.kPx = 39.8;
    w.kPairR = 3.94;
    w.kBss = 1.12;
    w.fixed = 1.0;
    w.canvasW = 1280;
    w.canvasH = 720;
    w.eLod = 0.0;
    w.eRes = 0.39;
    int floorSample = -1;
    int lodOnlyActs = 0;
    int acts = 0;
    for (int i = 0; i < 200; i++) {
      Governor::Sample s = w.step(gov);
      const double lodBefore = gov.lodBias();
      const double resBefore = gov.resolutionScale();
      if (gov.sample(cfg, s)) {
        acts++;
        if (gov.resolutionScale() == resBefore && gov.lodBias() != lodBefore) lodOnlyActs++;
      }
      if (floorSample < 0 && gov.resolutionScale() == kGovResScaleMin) floorSample = i;
    }
  }
}
`

// detectCommentNumber answers every number this rule reports in a comment.
func detectCommentNumber(c RuleCase) []ste.Finding {
	return caseFindings(c, commentfix.ID)
}

// autofixCommentNumber says the number in words, or names the item it counted.
func autofixCommentNumber(c RuleCase) RuleCase {
	return caseAutofix(c, commentfix.ID)
}
