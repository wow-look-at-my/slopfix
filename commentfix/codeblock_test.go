package commentfix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An indented run inside a doc comment is a code block: godoc renders it
// verbatim and a reader reads it as a table.
func indentedBlock(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "indented_block.golden"))
	require.NoError(t, err)
	return string(src)
}

func TestAnIndentedCodeBlockKeepsEveryLine(t *testing.T) {
	out := Fix("putback.go", indentedBlock(t)).Text
	for _, line := range []string{
		"//\tptbL the original parent directory, relative to the volume root",
		"//\tptbN the original name, which differs from the name in the trash when",
		"//\t     something was already called that",
	} {
		assert.Contains(t, out, line, "an indented line belongs to a code block and survives whole")
	}
}

func TestAnIndentedCodeBlockIsNotReflowed(t *testing.T) {
	out := FixLengthText(t, indentedBlock(t))
	require.NotEmpty(t, out)
	assert.NotContains(t, out, "root ptbN", "two lines joined into one")
}

// classSBlock is a doc comment whose tab-indented lines are Ruby, carrying
// digits that are code, not counts.
var classSBlock = []string{
	"//\tclass_name = name.capitalize",
	"//\tclass_name.gsub!(/[-_.\\s][a-zA-Z0-9]/) { |matched| matched.chars.fetch(-1).upcase }",
	"//\tclass_name.tr!(\"+\", \"x\")",
	"//\tclass_name.sub!(/(.)@(\\d)/, \"\\\\1AT\\\\2\")",
}

func classSSource(prose string) string {
	return "package brew\n\n" +
		prose +
		"//\n" +
		strings.Join(classSBlock, "\n") + "\n" +
		"func brewClassS(name string) string {\n\treturn name\n}\n"
}

func TestNumbersInADocCommentCodeBlockAreLeftAlone(t *testing.T) {
	src := classSSource("// brewClassS is Homebrew's Formulary.class_s:\n")
	assert.Empty(t, Check("brew.go", src), "a code block line states no count")
	got := Fix("brew.go", src)
	assert.False(t, got.Changed)
	assert.Equal(t, src, got.Text)
	assert.Empty(t, got.Removed)
}

func TestAProseNumberBesideACodeBlockIsStillRepaired(t *testing.T) {
	src := classSSource("// brewClassS is Homebrew's Formulary.class_s. It folds 3 separators:\n")
	hits := Check("brew.go", src)
	require.Len(t, hits, 1)
	assert.Equal(t, "3", hits[0].Number)
	assert.Equal(t, 3, hits[0].Line)

	got := Fix("brew.go", src)
	require.True(t, got.Changed)
	assert.NotContains(t, strings.Split(got.Text, "\n")[2], "3")
	assert.Contains(t, got.Text, strings.Join(classSBlock, "\n")+"\nfunc brewClassS", "the code block is byte-identical")
	assert.Empty(t, Check("brew.go", got.Text))
}

func TestNumbersInAHashCommentCodeBlockAreLeftAlone(t *testing.T) {
	src := "# Splits on the pattern:\n#\n#\tre.split(r\"[a-z0-9]\", s, maxsplit=1)\nx = 1\n"
	assert.Empty(t, Check("x.py", src))
	assert.Equal(t, src, Fix("x.py", src).Text)
}

// FixLengthText runs the length repair and answers what it wrote.
func FixLengthText(t *testing.T, src string) string {
	t.Helper()
	out, _ := FixLength("putback.go", src)
	if strings.TrimSpace(out) == "" {
		return src
	}
	return out
}
