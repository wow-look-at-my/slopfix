package slopfix_test

import (
	"go/format"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

// gofmt answers the layout gofmt writes for src.
func gofmt(t *testing.T, src string) string {
	t.Helper()
	out, err := format.Source([]byte(src))
	require.NoError(t, err)
	return string(out)
}

// alignedFields holds fields that a comment keeps apart, so gofmt does not align them.
const alignedFields = "package p\n\ntype pending struct {\n\t// The commit that waits for the gate.\n\tName string\n\t// Each take pays for 1 add.\n\tDescription string\n}\n"

// Each fixture starts in the gofmt layout, and the cut of a comment line alone
// took it out. The repair must write the file as gofmt does.
func TestARepairedGoFileKeepsTheGofmtLayout(t *testing.T) {
	for _, c := range []struct {
		name, path, src, cut string
	}{
		{"fields the cut brings together align", "x.go", alignedFields, "Each take"},
		{
			"a cut between the package clause and the imports leaves a single blank line",
			"x.go",
			"package runner\n\n// Each take pays for 1 add.\n\nimport (\n\t\"fmt\"\n)\n\nvar _ = fmt.Sprint\n",
			"Each take",
		},
		{
			"a cut at the end of a test file leaves no blank line before the end",
			"x_test.go",
			"package p\n\nfunc f() {}\n\n// A trailing note nobody attached to any code at all, sitting at the end of the file and documenting nothing whatsoever.\n",
			"A trailing note",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, gofmt(t, c.src), c.src, "the fixture starts in the gofmt layout")
			repair := slopfix.Fix(slopfix.Request{Content: c.src, Path: c.path})
			require.True(t, repair.Changed)
			assert.NotContains(t, repair.Text, c.cut)
			assert.Equal(t, gofmt(t, repair.Text), repair.Text, "gofmt rewrites what the repair wrote")
		})
	}
}

// A caller that names a single rule still gets a file gofmt accepts.
func TestANamedRuleStillLeavesTheGofmtLayout(t *testing.T) {
	repair := slopfix.Fix(slopfix.Request{
		Content: alignedFields,
		Path:    "x.go",
		Rules:   []slopfix.Rule{slopfix.RuleComments},
		IDs:     []string{"comments/number"},
	})
	require.True(t, repair.Changed)
	assert.Equal(t, gofmt(t, repair.Text), repair.Text)
}

// A file a repair changes comes back in the gofmt layout, whatever layout it had.
func TestARepairedGoFileTakesTheGofmtLayout(t *testing.T) {
	src := "package p\n\n\n\n// Each take pays for 1 add.\nvar x   = 1\n"
	repair := slopfix.Fix(slopfix.Request{Content: src, Path: "x.go"})
	assert.Equal(t, "package p\n\nvar x = 1\n", repair.Text)
}

// A file no rule changes comes back byte for byte, in whatever layout it has.
func TestAGoFileNoRuleChangesIsNotReformatted(t *testing.T) {
	src := "package p\n\n\n\nfunc f()   {}\n"
	repair := slopfix.Fix(slopfix.Request{Content: src, Path: "x.go"})
	assert.False(t, repair.Changed)
	assert.Equal(t, src, repair.Text)
}

// Only Go has a layout to restore. A cut in another language leaves its blanks alone.
func TestALayoutPassIsForGoAlone(t *testing.T) {
	src := "fn f() {}\n\n// Each take pays for 1 add.\n\n\nfn g() {}\n"
	repair := slopfix.Fix(slopfix.Request{Content: src, Path: "x.rs"})
	assert.Equal(t, "fn f() {}\n\n\n\nfn g() {}\n", repair.Text)
}
