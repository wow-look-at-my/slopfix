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

func TestAnIndentedCodeBlockKeepsEveryRow(t *testing.T) {
	out := Fix("putback.go", indentedBlock(t)).Text
	for _, row := range []string{
		"//\tptbL the original parent directory, relative to the volume root",
		"//\tptbN the original name, which differs from the name in the trash when",
		"//\t     something was already called that",
	} {
		assert.Contains(t, out, row, "an indented row is a code block line and survives whole")
	}
}

func TestAnIndentedCodeBlockIsNotReflowed(t *testing.T) {
	out := FixLengthText(t, indentedBlock(t))
	require.NotEmpty(t, out)
	assert.NotContains(t, out, "root ptbN", "two rows joined into one line")
}

// classSBlock is a doc comment whose tab-indented rows are Ruby, carrying
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
	assert.Empty(t, Check("brew.go", src), "a code block row states no count")
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

// displayShapeRow is the code row of ffs.impl.bash's displayShape comment.
const displayShapeRow = "//\te<Name>;  enum   l<shape>  list of shape   m<key><val>  map   .  anything else"

// displayShapeSource is that comment over its function. The comment runs
// longer than the code, so the length repair cuts it.
const displayShapeSource = "package codegen\n\n" +
	"// displayShape encodes where enums sit inside a type, for the runtime's\n" +
	"// display walk. Empty means the type holds no enum, and the ordinary\n" +
	"// rendering applies. Encoding (prefix form, parses left to right):\n" +
	"//\n" +
	displayShapeRow + "\n" +
	"//\n" +
	"// Struct interiors are NOT encoded here: a struct renders through its own\n" +
	"// generated field-shape metadata, so a recursive type cannot run away.\n" +
	"func displayShape(t ir.TypeRef) string {\n" +
	"\tif t == nil {\n" +
	"\t\treturn \"\"\n" +
	"\t}\n" +
	"\tswitch t.Kind {\n" +
	"\tcase ir.TEnum:\n" +
	"\t\treturn \"e\" + bashSym(t.Name) + \";\"\n" +
	"\tcase ir.TList:\n" +
	"\t\tif inner := displayShape(t.Elem); inner != \"\" {\n" +
	"\t\t\treturn \"l\" + inner\n" +
	"\t\t}\n" +
	"\tcase ir.TMap:\n" +
	"\t\tk, v := displayShape(t.KeyType), displayShape(t.ValType)\n" +
	"\t\tif k != \"\" || v != \"\" {\n" +
	"\t\t\tif k == \"\" {\n" +
	"\t\t\t\tk = \".\"\n" +
	"\t\t\t}\n" +
	"\t\t\tif v == \"\" {\n" +
	"\t\t\t\tv = \".\"\n" +
	"\t\t\t}\n" +
	"\t\t\treturn \"m\" + k + v\n" +
	"\t\t}\n" +
	"\t}\n" +
	"\treturn \"\"\n" +
	"}\n"

// A cut keeps the opening of the block and never joins a code row into prose.
// Joined, the row read "map. anything else Struct interiors are NOT encoded
// here.", which is no sentence.
func TestACutNeverJoinsACodeRowIntoProse(t *testing.T) {
	require.NotEmpty(t, CheckLength("alloc.go", displayShapeSource), "the comment runs longer than its code")
	out, changed := FixLength("alloc.go", displayShapeSource)
	require.True(t, changed)

	assert.NotContains(t, out, "map. anything else")
	assert.NotContains(t, out, "e<Name>; enum")
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "e<Name>") {
			assert.Equal(t, displayShapeRow, line, "a code row survives whole or not at all")
		}
	}
	assert.Contains(t, out, "// displayShape encodes where enums sit inside a type")
	assert.Empty(t, CheckLength("alloc.go", out))
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
