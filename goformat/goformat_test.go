package goformat_test

import (
	"go/format"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/goformat"
)

func gofmt(t *testing.T, src string) string {
	t.Helper()
	out, err := format.Source([]byte(src))
	require.NoError(t, err)
	return string(out)
}

func TestTheEditsGiveTheGofmtLayout(t *testing.T) {
	for _, src := range []string{
		"package p\n\ntype t struct {\n\tA string\n\tLonger int\n}\n",
		"package p\n\n\nimport \"fmt\"\n\nvar _ = fmt.Sprint\n\n",
		"package p\n\nfunc f()   { x:=1; _ = x }\n",
		"package p\r\n\r\nvar x = 1 // note   \r\n",
	} {
		res := goformat.Gate(src, goformat.Edits(src), edit.Scope{})
		assert.Empty(t, res.Refused, src)
		assert.Equal(t, gofmt(t, src), res.Text, src)
	}
}

// Only the org's Go fork parses a parameter default. This test fails when the build links upstream go/format.
func TestTheLayoutComesFromTheOrgGoFork(t *testing.T) {
	src := "package p\n\n\nfunc f(n int = 3) int { return n }\n"
	res := goformat.Gate(src, goformat.Edits(src), edit.Scope{})
	assert.Empty(t, res.Refused)
	assert.Equal(t, "package p\n\nfunc f(n int = 3) int { return n }\n", res.Text)
}

func TestNothingToDoAnswersNoEdit(t *testing.T) {
	for name, src := range map[string]string{
		"the gofmt layout already":   "package p\n\nvar x = 1\n",
		"a fragment with no package": "x  :=  1\n",
		"source that does not parse": "package p\n\nfunc (\n",
		"a layout past whitespace":   "package p\n\nvar  x = 1;\n",
		"whitespace ahead of a cut":  "package p\n\nvar  x = 1\nvar y = 2;\n",
	} {
		assert.Empty(t, goformat.Edits(src), name)
	}
}

// blankAfter is the edit that writes text over the blanks after the first want in src.
func blankAfter(src, want, text string) edit.Edit {
	start := strings.Index(src, want) + len(want)
	end := start
	for end < len(src) && src[end] == ' ' {
		end++
	}
	return edit.Edit{Start: start, End: end, Text: text}
}

// Each edit changes what the scanner reads. The word fails the blank test, and
// the rest trade blanks for blanks.
func TestTheGateRefusesWhatChangesTheTokens(t *testing.T) {
	word := "package p\n\nvar x = 1\n"
	quoted := "package p\n\nvar s = \"a  b\"\n"
	sum := "package p\n\nvar x = a + b\n"
	directive := "package p\n\n//go:generate true\nvar x = 1\n"
	for name, c := range map[string]struct {
		src string
		e   edit.Edit
	}{
		"a word":                        {word, edit.Edit{Start: len("package p\n\nvar "), End: len("package p\n\nvar x"), Text: "y"}},
		"a blank inside a string":       {quoted, blankAfter(quoted, "\"a", " ")},
		"a line end that ends a line":   {sum, blankAfter(sum, "= a", "\n")},
		"a blank that ends a directive": {directive, blankAfter(directive, "//", " ")},
	} {
		res := goformat.Gate(c.src, []edit.Edit{c.e}, edit.Scope{})
		assert.Equal(t, c.src, res.Text, name)
		assert.Len(t, res.Refused, 1, name)
	}
}

func TestTheGateKeepsItsEditsInsideTheScope(t *testing.T) {
	src := "package p\n\n\nvar x   = 1\n"
	inside := strings.Index(src, "x") + 1
	res := goformat.Gate(src, goformat.Edits(src), edit.Within(inside, len(src)))
	assert.Equal(t, "package p\n\n\nvar x = 1\n", res.Text)
	assert.Empty(t, res.Refused)
}
