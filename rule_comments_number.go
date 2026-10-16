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
			// A zero that is the subject of a verb, as in buildhost's upload.ts.
			{Name: commentfix.ID + "-zero-subject", Path: "size.ts", Text: "/** The size from the input: empty is the default, 0 disables chunking. */\nexport function size(input: string): number {\n\treturn input.length;\n}\n"},
			// A floor stays a minimum: cutting the number alone would write "at least nouns".
			{
				Name: commentfix.ID + "-floor", Path: "main.go",
				Text: "package main\n\n// A run of at least two nouns stands together.\nfunc main() {}\n",
				Want: "package main\n\n// A run of multiple nouns stands together.\nfunc main() {}\n",
			},
			// A "more than" stays true for every cardinal it can name.
			{
				Name: commentfix.ID + "-more-than", Path: "main.go",
				Text: "package main\n\n// A run of more than three nouns stands together.\nfunc main() {}\n",
				Want: "package main\n\n// A run of more than a couple of nouns stands together.\nfunc main() {}\n",
			},
			// A ceiling stays a maximum.
			{
				Name: commentfix.ID + "-ceiling", Path: "main.go",
				Text: "package main\n\n// A run of at most three nouns stands together.\nfunc main() {}\n",
				Want: "package main\n\n// A run of a bounded number of nouns stands together.\nfunc main() {}\n",
			},
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
