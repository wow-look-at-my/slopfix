package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A comment with no code under it has nothing to be measured against, so no cut
// ever brings it inside a budget. It was reported on every run and repaired on
// none, which leaves a reader hand-editing prose or deleting the file.
//
// The rule's verdict is that it documents nothing, and the repair takes it at
// its word.
func TestACommentDocumentingNothingLosesItsLines(t *testing.T) {
	src := "package p\n\nfunc f() {}\n\n// A trailing note nobody attached to any code at all, sitting at the end of the file and documenting nothing whatsoever.\n"

	hits := Check("x.go", src)
	require.Len(t, hits, 1)
	assert.Equal(t, "the comment documents nothing", hits[0].Tell)
	assert.True(t, hits[0].Repairable)

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.NotContains(t, out, "A trailing note")
	assert.Contains(t, out, "func f() {}")
	assert.Empty(t, Check("x.go", out))
}

// The same inside a function body, where the code is above the comment rather
// than below it.
func TestATrailingCommentInsideABodyLosesItsLines(t *testing.T) {
	src := "package p\n\nfunc f() {\n\tg()\n\t// Nothing follows this, so it documents nothing at all and no cut can ever fit it to a budget.\n}\n"

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Contains(t, out, "g()")
	assert.Empty(t, Check("x.go", out))
}

// A directive in the run is an instruction the prose beside it explains, so it
// is what the prose is weighed against. A gen.go carries nothing but a
// //go:generate line and the paragraph saying why, and the deletion above
// would have taken that paragraph on every run.
func TestProseBesideADirectiveIsWeighedAgainstIt(t *testing.T) {
	src := "package p\n\n// A module zip carries the gitlink and none of the submodule's files, so a\n" +
		"// consumer has to fetch the sources before anything can translate them.\n" +
		"//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-go -dir testdata/tree-sitter-go\n" +
		"//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package golang -out parser.gen.go testdata/tree-sitter-go/src/parser.c\n"

	assert.Empty(t, Check("x.go", src))
	_, changed := Fix("x.go", src)
	assert.False(t, changed)
}

// A directive addresses a tool rather than a reader, and a lost //go:embed
// leaves the variable it filled empty. So a run whose prose outweighs even its
// directive keeps the directive and loses the prose.
func TestADirectiveSurvivesTheCut(t *testing.T) {
	src := "package p\n\nfunc f() {}\n\n//go:debug x=1\n" +
		"// A trailing paragraph nobody attached to any code at all, sitting at the\n" +
		"// end of the file, running several lines past anything it could be weighed\n" +
		"// against, and documenting nothing whatsoever for the reader who finds it.\n"

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Contains(t, out, "//go:debug x=1")
	assert.Empty(t, Check("x.go", out))
}

// The control. A comment that DOES document code is cut back to fit rather than
// deleted, so the case above is about the measure and not about comments.
func TestACommentOverItsBudgetIsCutRatherThanDeleted(t *testing.T) {
	src := "package p\n\n// The point.\n//\n// Then a paragraph of elaboration that runs well past the length of the\n// declaration it sits above, several lines of it, saying little.\nconst a = 1\n"

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Contains(t, out, "// The point.")
	assert.Equal(t, 1, strings.Count(out, "const a = 1"))
	assert.Empty(t, Check("x.go", out))
}
