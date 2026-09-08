package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These pin the rule against the check it replaces, go-toolchain's commentspan
// analyzer. A verdict that differs from it is a regression for every repository
// that switches over, so each case here pins a stated property of it.

// The measures are independent, and a block can fail both together. Reporting
// only the leading tell hides half of what is wrong with the block.
func TestBothMeasuresAreReportedTogether(t *testing.T) {
	long := "// " + strings.Repeat("elaboration that carries real weight ", 5)
	src := strings.Join([]string{
		"package p",
		"",
		"// The point.",
		long,
		"const p = 1",
	}, "\n")

	hits := Check("x.go", src)
	require.Len(t, hits, 1)
	assert.Contains(t, hits[0].Tell, "more lines")
	assert.Contains(t, hits[0].Tell, "longer than")
}

// Indentation carries no cost, on either side. A comment inside a deeply
// nested block is not an essay for being indented.
func TestIndentationIsNotWeighed(t *testing.T) {
	body := func(indent string) string {
		return strings.Join([]string{
			"package p",
			"",
			"func f() {",
			indent + "// A note that is comfortably in proportion here.",
			indent + "call(a, b, c)",
			indent + "call(d, e, f)",
			"}",
		}, "\n")
	}
	assert.Equal(t, Check("x.go", body("\t")), Check("x.go", body("\t\t\t\t")))
}

// A build constraint is an instruction to a tool. Measuring it reports a block
// nobody wrote as prose.
func TestADirectiveLineIsNotProse(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"//go:generate stringer -type=Kind -linecomment -output kind_string.go",
		"//go:generate mockgen -source=kind.go -destination=mock_kind.go",
		"type Kind int",
	}, "\n")
	assert.Empty(t, Check("x.go", src), "a run of directives is not a comment block")
}

// A sentence carrying a colon is prose, whatever follows the colon. Reading it
// as a directive would drop real text out of the measurement.
func TestASentenceWithAColonIsStillProse(t *testing.T) {
	long := "// Never derive this: the server owns the URL grammar, and building it " +
		"here means owning a copy of that grammar for the rest of time."
	src := "package p\n\n" + long + "\nconst u = \"x\"\n"
	assert.NotEmpty(t, Check("x.go", src), "the colon does not make this a directive")
}

// The package doc introduces the file rather than a declaration, so there is
// nothing of comparable size to weigh it against.
func TestThePackageDocIsNeverMeasured(t *testing.T) {
	doc := strings.Repeat("// A long package comment that runs for a while.\n", 8)
	src := doc + "package p\n\nconst p = 1\n"
	assert.Empty(t, Check("x.go", src))
}

// The control: the same prose over a declaration IS measured, which proves the
// case above is skipping the package doc rather than missing every block.
func TestTheSameProseOverADeclarationIsMeasured(t *testing.T) {
	doc := strings.Repeat("// A long comment that runs for a while.\n", 8)
	src := "package p\n\n" + doc + "const p = 1\n"
	assert.NotEmpty(t, Check("x.go", src))
}

// Every comment gets a floor of characters whatever it documents, so a short
// note over a short line is never a finding.
func TestTheCharacterFloorHolds(t *testing.T) {
	note := "// " + strings.Repeat("a", floorChars-10)
	src := "package p\n\n" + note + "\nconst p = 1\n"
	assert.Empty(t, Check("x.go", src), "a comment inside the floor is allowed")

	over := "// " + strings.Repeat("a", floorChars+40)
	assert.NotEmpty(t, Check("x.go", "package p\n\n"+over+"\nconst p = 1\n"))
}
