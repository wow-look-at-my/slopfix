// comments/length: a comment run weighed against the construct beneath it.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// comments/length: a comment run weighed against the construct beneath it. The
// repair cuts from the end, because a comment leads with its point.
func init() {
	RegisterRule(RuleSpec{
		ID:       commentfix.IDLength,
		Category: RuleComments,
		Detect:   detectCommentLength,
		Autofix:  autofixCommentLength,
		Cases: []RuleCase{
			{Name: commentfix.IDLength, Path: "main.go", Text: fixtureHeader + "package main\n\n" + overlongComment() + "func main() {}\n"},
			{Name: "a sentence that opens on an identifier is cut whole", Path: "froxel-path.cpp", Text: fixtureHeader + identifierSentenceCase, Want: fixtureHeader + identifierSentenceWant},
			{Name: "a run of doc comments is cut from its end", Path: "snapshot.ts", Text: fixtureHeader + stackedDocCase},
			{Name: "an opener inside a block comment is prose", Path: "records.ts", Text: fixtureHeader + innerOpenerCase},
		},
	})
}

//go:embed testdata/identifier-sentence-case.cpp
var identifierSentenceCase string

//go:embed testdata/identifier-sentence-want.cpp
var identifierSentenceWant string

// stackedDocCase holds a doc comment left above another doc comment. The run
// of both outweighs the function beneath it.
//
//go:embed testdata/stacked-doc-case.ts
var stackedDocCase string

// innerOpenerCase holds a block comment whose prose names a glob ending in a
// comment opener. The comment still closes once, at its end.
//
//go:embed testdata/inner-opener-case.ts
var innerOpenerCase string

// detectCommentLength answers every overlong comment run this rule reports.
func detectCommentLength(c RuleCase) []ste.Finding {
	return caseFindings(c, commentfix.IDLength)
}

// autofixCommentLength cuts the run from its end down to the budget.
func autofixCommentLength(c RuleCase) RuleCase {
	return caseAutofix(c, commentfix.IDLength)
}

// overlongComment is a Go comment far longer than the code beneath it, and
// under the volume cap. Only its weight against the code reads it.
func overlongComment() string {
	return "// The loop reads each value from the input and adds it to the total.\n" +
		"// It then checks the total against the limit that the caller set.\n" +
		"// A total over the limit stops the loop before it writes anything.\n" +
		"//\n" +
		"// The caller reads the total after the loop stops.\n" +
		"// The caller then writes the total to the log.\n"
}
