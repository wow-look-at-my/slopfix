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
			// A label after a subordinator names an item, and the word after it is the verb of its clause.
			{Name: commentfix.ID + " in a label after a subordinator", Path: "main.go", Text: "package main\n\n// Work begins only after instance 1 finishes.\nfunc main() {}\n", Want: "package main\n\n// Work begins only after one instance finishes.\nfunc main() {}\n"},
			// sglang .pre-commit-config.yaml: an issue reference goes with the preposition before it, so no "in #." is left.
			{Name: commentfix.ID + " on an issue reference", Path: ".pre-commit-config.yaml", Text: "repos:\n  - repo: local\n    hooks:\n      - id: forbid-docs-new-path\n        # docs_new/ was renamed to docs/ in #32123.\n        language: fail\n"},
			// A zero that is the subject of a verb, as in buildhost's upload.ts.
			{Name: commentfix.ID + "-zero-subject", Path: "size.ts", Text: "/** The size from the input: empty is the default, 0 disables chunking. */\nexport function size(input: string): number {\n\treturn input.length;\n}\n"},
		},
	})
}

// detectCommentNumber answers every number this rule reports in a comment.
func detectCommentNumber(c RuleCase) []ste.Finding {
	return caseFindings(c, commentfix.ID)
}

// autofixCommentNumber says the number in words, or names the item it counted.
func autofixCommentNumber(c RuleCase) RuleCase {
	return caseAutofix(c, commentfix.ID)
}
