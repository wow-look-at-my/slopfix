package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A comment with no code under it gets the budget of a construct of one line.
// The repair cuts it to that budget and keeps its opening sentence.
func TestACommentDocumentingNothingKeepsItsOpening(t *testing.T) {
	src := "package p\n\nfunc f() {}\n\n// A trailing note sits at the end of the file.\n" +
		"// It documents no code, and it runs on past the budget of a single line of code.\n"

	hits := CheckLength("x.go", src)
	require.Len(t, hits, 1)
	assert.Equal(t, "the comment documents nothing", hits[0].Tell)
	assert.True(t, hits[0].Repairable)

	out, changed := FixLength("x.go", src)
	require.True(t, changed)
	assert.Equal(t, "package p\n\nfunc f() {}\n\n// A trailing note sits at the end of the file.\n", out)
	assert.Empty(t, CheckLength("x.go", out))
}

// A single sentence under the cap fits the budget of one line. A comment
// over no code that says only that is not a finding.
func TestATrailingCommentInsideABodyStays(t *testing.T) {
	src := "package p\n\nfunc f() {\n\tg()\n\t// Nothing follows this, so it documents nothing at all and no cut can ever fit it to a budget.\n}\n"

	assert.Empty(t, CheckLength("x.go", src))
	out, changed := FixLength("x.go", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
}

// A directive in the run is an instruction the prose beside it explains, so it
// is what the prose is weighed against. A gen.go carries nothing but a
// //go:generate line and the paragraph saying why, and the deletion above
// would have taken that paragraph. This happens on every run.
func TestProseBesideADirectiveIsWeighedAgainstIt(t *testing.T) {
	src := "package p\n\n// A module zip carries the gitlink and none of the submodule's files, so a\n" +
		"// consumer has to fetch the sources before anything can translate them.\n" +
		"//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-go -dir testdata/tree-sitter-go\n" +
		"//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package golang -out parser.gen.go testdata/tree-sitter-go/src/parser.c\n"

	assert.Empty(t, CheckLength("x.go", src))
	_, changed := FixLength("x.go", src)
	assert.False(t, changed)
}

// A directive addresses a tool rather than a reader, and a lost //go:embed
// leaves the variable it filled empty. So a cut keeps the directive, and the
// prose keeps its opening.
func TestADirectiveSurvivesTheCut(t *testing.T) {
	src := "package p\n\nfunc f() {}\n\n//go:debug x=1\n" +
		"// A trailing paragraph nobody attached to any code at all, sitting at the\n" +
		"// end of the file, running several lines past anything it could be weighed\n" +
		"// against, and documenting nothing whatsoever for the reader who finds it.\n"

	out, _ := FixLength("x.go", src)
	assert.Contains(t, out, "//go:debug x=1")
	assert.Contains(t, out, "// A trailing paragraph", "the prose lost its opening:\n%s", out)
	assertWholeSTESentences(t, out)
}

// The control. A comment that DOES document code is cut back to fit rather than
// deleted. The case above is about the measure and not about comments.
func TestACommentOverItsBudgetIsCutRatherThanDeleted(t *testing.T) {
	src := "package p\n\n// The point.\n//\n// Then a paragraph of elaboration that runs well past the length of the\n// declaration it sits above, several lines of it, saying little.\nconst a = 1\n"

	out, changed := FixLength("x.go", src)
	require.True(t, changed)
	assert.Contains(t, out, "// The point.")
	assert.Equal(t, 1, strings.Count(out, "const a = 1"))
	assert.Empty(t, CheckLength("x.go", out))
}

// A TypeScript triple-slash directive is read by the compiler. The vite
// declaration file holds nothing else, and the directive is not a comment.
func TestATripleSlashDirectiveDocumentsNothingAndStays(t *testing.T) {
	src := "/// <reference types=\"vite/client\" />\n"
	assert.Empty(t, CheckLength("src/vite-env.d.ts", src))
	out, changed := FixLength("src/vite-env.d.ts", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
}
