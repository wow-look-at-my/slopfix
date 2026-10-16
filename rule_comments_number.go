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
				Text: "# Each run must exit 0, report the registry slots and the stripe mode, report 0 validation errors and write its PNG.\nset(A 1)\n",
				Want: "# Each run must exit 0, report the registry slots and the stripe mode, report no validation errors and write its PNG.\nset(A 1)\n",
			},
			{
				Name: "an exit status wrapped onto the next comment line", Path: "gate.h", Unchanged: true,
				Text: "// Under --validate the validation errors of the device phases fail the run (exit\n// 2), the convention of the shed test.\nint runRecreateTest() {\n\tint failed = validate();\n\treturn failed;\n}\n",
			},
			{
				Name: "a comment after code continues on the aligned lines under it", Path: "opts.h",
				Text: "struct Options {\n" +
					"  bool sortShuffle = true;    // --sort-impl picks the sort the raster runs.\n" +
					"  bool rasterStats = false;   // --raster-stats 0|1: compile the counters of\n" +
					"                              //   the raster in. The image bytes stay the same.\n" +
					"  double pairBudgetMb = 0.0;  // --pair-budget-mb caps the pair buffer.\n" +
					"};\n",
				Want: "struct Options {\n" +
					"  bool sortShuffle = true;    // --sort-impl picks the sort the raster runs.\n" +
					"  bool rasterStats = false;   // The image bytes stay the same.\n" +
					"  double pairBudgetMb = 0.0;  // --pair-budget-mb caps the pair buffer.\n" +
					"};\n",
			},
			{
				Name: "a rule line that frames a banner stays a line of its own", Path: "banner.h",
				Text: bannerOpen + "// The gate renders the scene in each mode and compares the bytes of the frames of every run against the baseline.\n// There are 3 modes today.\n" + bannerClose,
				Want: bannerOpen + "// The gate renders the scene in each mode and compares the bytes of the frames of every run against the baseline.\n// There are modes today.\n" + bannerClose,
			},
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

// detectCommentNumber answers every number this rule reports in a comment.
func detectCommentNumber(c RuleCase) []ste.Finding {
	return caseFindings(c, commentfix.ID)
}

// autofixCommentNumber says the number in words, or names the item it counted.
func autofixCommentNumber(c RuleCase) RuleCase {
	return caseAutofix(c, commentfix.ID)
}
