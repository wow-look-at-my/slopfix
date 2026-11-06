package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// The control. A comment that DOES document code is cut back to fit rather than
// deleted. The nothing.go case of rules/comment-repair.xml is about the measure.
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
