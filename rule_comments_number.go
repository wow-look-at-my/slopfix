// rule_comments_number.go holds the rule comments number code.

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
			{Name: commentfix.ID, Path: "main.go", Text: "package main\n\n// There are 3 modes today.\nfunc main() {}\n"},
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

// cutParagraphCase opens with a paragraph of a single sentence the number cut takes whole.
//
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

// valuesCase holds a count beside the value of a flag, a set literal and an assignment.
//
//go:embed testdata/values-case.cpp
var valuesCase string

//go:embed testdata/values-want.cpp
var valuesWant string

// frameZeroCase names frame zero at the start of a sentence and before a possessive ending.
//
//go:embed testdata/frame-zero-case.cpp
var frameZeroCase string

//go:embed testdata/frame-zero-want.cpp
var frameZeroWant string

// ruledNumbersCase opens every comment line with a rule of dashes.
//
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
