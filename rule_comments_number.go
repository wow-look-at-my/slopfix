// comments/number: a number in a comment is a count of what exists today.

package slopfix

import _ "embed"

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
			{Name: commentfix.ID, Path: "main.go", Text: fixtureHeader + "package main\n\n// There are 3 modes today.\nfunc main() {}\n"},
			// A GLSL shader reads through the C grammar, so its comments are judged too.
			{Name: commentfix.ID + "-glsl", Path: "froxel-raster.comp", Text: fixtureHeader + "#version 450\nlayout(local_size_x = 16) in;\nlayout(std430, set = 0, binding = 1) readonly buffer Projected { float projected[]; };\n\n// There are 3 modes today.\nvoid main() {}\n"},
			{
				Name: "a verb before a zero count is no label", Path: "gate.cmake",
				Text: fixtureHeader + "# Each run must exit 0, report the cache entries and the retry mode, report 0 validation errors and write its log.\nset(A 1)\n",
				Want: fixtureHeader + "# Each run must exit 0, report the cache entries and the retry mode, report no validation errors and write its log.\nset(A 1)\n",
			},
			{
				Name: "an exit status wrapped onto the next comment line", Path: "gate.h",
				Text: fixtureHeader + embeddedCpp2,
				Want: fixtureHeader + embeddedCpp3,
			},
			{
				Name: "a comment after code continues on the aligned lines under it", Path: "opts.h",
				Text: fixtureHeader + "struct Options {\n" +
					"  bool tracePrefix = true;    // --prefix-mode picks the prefix the trace uses.\n" +
					"  bool traceCounts = false;   // --trace-counts 0|1: compile the counters of\n" +
					"                              //   the trace in. The log bytes stay the same.\n" +
					"  double poolBudgetMb = 0.0;  // --pool-budget-mb caps the pool buffer.\n" +
					"};\n",
				Want: fixtureHeader + "struct Options {\n" +
					"  bool tracePrefix = true;    // --prefix-mode picks the prefix the trace uses.\n" +
					"  bool traceCounts = false;   // The log bytes stay the same.\n" +
					"  double poolBudgetMb = 0.0;  // --pool-budget-mb caps the pool buffer.\n" +
					"};\n",
			},
			{
				Name: "a rule line that frames a banner stays a line of its own", Path: "banner.h",
				Text: fixtureHeader + bannerOpen + "// The gate renders the scene in each mode and compares the bytes of the frames of every run against the baseline.\n// There are 3 modes today.\n" + bannerClose,
				Want: fixtureHeader + bannerOpen + "// The gate renders the scene in each mode and compares the bytes of the frames of every run against the baseline.\n// There are modes today.\n" + bannerClose,
			},
			{Name: "a cut keeps the rule that opens each line", Path: "test-governor.cpp", Text: fixtureHeader + ruledNumbersCase, Want: fixtureHeader + ruledNumbersWant},
			{Name: "index zero after a possessive is the first item", Path: "froxel-path.h", Text: fixtureHeader + chunkZeroCase, Want: fixtureHeader + chunkZeroWant},
			{Name: "index zero is the first item", Path: "froxel-path.cpp", Text: fixtureHeader + frameZeroCase, Want: fixtureHeader + frameZeroWant},
			{Name: "a value is cut with its sentence", Path: "warp-pass.h", Text: fixtureHeader + valuesCase, Want: fixtureHeader + valuesWant},
			{Name: "a cut paragraph takes its break with it", Path: "warp-pass.h", Text: fixtureHeader + cutParagraphCase, Want: fixtureHeader + cutParagraphWant},
			// A label after a subordinator names an item, and the word after it is the verb of its clause.
			{Name: commentfix.ID + " in a label after a subordinator", Path: "main.go", Text: fixtureHeader + "package main\n\n// Work begins only after instance 1 finishes.\nfunc main() {}\n", Want: fixtureHeader + "package main\n\n// Work begins only after one instance finishes.\nfunc main() {}\n"},
			// sglang .pre-commit-config.yaml: an issue reference goes with the preposition before it, so no "in #." is left.
			{Name: commentfix.ID + " on an issue reference", Path: ".pre-commit-config.yaml", Text: issueReferenceYml},
			// A zero that is the subject of a verb, as in buildhost's upload.ts.
			{Name: commentfix.ID + "-zero-subject", Path: "size.ts", Text: "/** The size from the input: empty is the default, 0 disables chunking. */\nexport function size(input: string): number {\n\treturn input.length;\n}\n"},
			// A floor stays a minimum: cutting the number alone would write "at least nouns".
			{
				Name: commentfix.ID + "-floor", Path: "main.go",
				Text: fixtureHeader + "package main\n\n// A run of at least two nouns stands together.\nfunc main() {}\n",
				Want: fixtureHeader + "package main\n\n// A run of at least a couple of nouns stands together.\nfunc main() {}\n",
			},
			// A "more than" stays true for every cardinal it can name.
			{
				Name: commentfix.ID + "-more-than", Path: "main.go",
				Text: fixtureHeader + "package main\n\n// A run of more than three nouns stands together.\nfunc main() {}\n",
				Want: fixtureHeader + "package main\n\n// A run of more than a few nouns stands together.\nfunc main() {}\n",
			},
			// A ceiling stays a maximum.
			{
				Name: commentfix.ID + "-ceiling", Path: "main.go",
				Text: fixtureHeader + "package main\n\n// A run of at most three nouns stands together.\nfunc main() {}\n",
				Want: fixtureHeader + "package main\n\n// A run of a bounded number of nouns stands together.\nfunc main() {}\n",
			},
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

//go:embed testdata/cut-paragraph-case.cpp
var cutParagraphCase string

//go:embed testdata/cut-paragraph-want.cpp
var cutParagraphWant string

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

//go:embed testdata/values-case.cpp
var valuesCase string

//go:embed testdata/values-want.cpp
var valuesWant string

//go:embed testdata/frame-zero-case.cpp
var frameZeroCase string

//go:embed testdata/frame-zero-want.cpp
var frameZeroWant string

//go:embed testdata/ruled-numbers-case.cpp
var ruledNumbersCase string

//go:embed testdata/ruled-numbers-want.cpp
var ruledNumbersWant string

// detectCommentNumber answers every number this rule reports in a comment.
func detectCommentNumber(c RuleCase) []ste.Finding {
	return caseFindings(c, commentfix.ID)
}

// autofixCommentNumber says the number in words, or names the item it counted.
func autofixCommentNumber(c RuleCase) RuleCase {
	return caseAutofix(c, commentfix.ID)
}

//go:embed embedded.cpp
var embeddedCpp2 string

//go:embed embedded-2.cpp
var embeddedCpp3 string

//go:embed testdata/issue-reference.yml
var issueReferenceYml string
