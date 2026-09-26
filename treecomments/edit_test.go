package treecomments

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ts "github.com/wow-look-at-my/go-tree-sitter"
	"github.com/wow-look-at-my/slopfix/edit"
)

// commentEdit replaces the whole of the comment that holds needle.
func commentEdit(t *testing.T, filename, src, needle, text string) edit.Edit {
	t.Helper()
	for _, c := range Extract(filename, src) {
		if strings.Contains(c.Text, needle) {
			return edit.Edit{Start: c.Offset, End: c.Offset + len(c.Text), Text: text}
		}
	}
	require.FailNow(t, "no comment holds "+needle)
	return edit.Edit{}
}

const goSrc = "package p\n\n// Answer returns the answer.\nfunc Answer() int { return 42 }\n\n/* block */\nvar x = 1\n"

func TestARewriteThatStaysACommentLands(t *testing.T) {
	e := commentEdit(t, "p.go", goSrc, "Answer returns", "// Answer returns it.")
	res := Apply("p.go", goSrc, []edit.Edit{e}, edit.Scope{})
	assert.Empty(t, res.Refused)
	assert.Contains(t, res.Text, "// Answer returns it.\nfunc Answer()")
}

// Each rewrite below would carry text out of the comment it replaces. The
// gate must refuse every one, and leave the source as it was.
func TestARewriteCannotEscapeItsComment(t *testing.T) {
	cases := map[string]struct{ needle, text string }{
		"a newline ends a line comment":        {"Answer returns", "// Answer returns.\nfunc Evil() {}"},
		"a carriage return and newline":        {"Answer returns", "// Answer.\r\nvar evil = 1"},
		"a closer ends a block comment early":  {"block", "/* block */ var evil = 1 /* */"},
		"an opener swallows the code below":    {"Answer returns", "/* Answer returns"},
		"a line comment turned into code":      {"Answer returns", "Answer returns the answer."},
		"a string quote left open":             {"block", "/* ok */ \""},
		"an empty replacement joins two lines": {"block", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := commentEdit(t, "p.go", goSrc, c.needle, c.text)
			res := Apply("p.go", goSrc, []edit.Edit{e}, edit.Scope{})
			if len(res.Applied) == 1 {
				// A rewrite that lands must still hold every code token as it was.
				assert.Equal(t, shape(mustParse(t, goSrc), goSrc), shape(mustParse(t, res.Text), res.Text), "the code tree changed")
				return
			}
			assert.Len(t, res.Refused, 1, "the escape was not refused")
			assert.Equal(t, goSrc, res.Text)
		})
	}
}

func mustParse(t *testing.T, src string) ts.Node {
	t.Helper()
	root, ok := parseWith(languageFor("p.go"), src)
	require.True(t, ok)
	return root
}

// An edit that covers a byte of code is refused before anything is written.
func TestAnEditOverCodeIsRefused(t *testing.T) {
	at := strings.Index(goSrc, "func Answer")
	res := Apply("p.go", goSrc, []edit.Edit{{Start: at, End: at + 4, Text: "func"}}, edit.Scope{})
	require.Len(t, res.Refused, 1)
	assert.Contains(t, res.Refused[0].Reason, "code")
	assert.Equal(t, goSrc, res.Text)
}

// A marker inside a string literal is no comment, so an edit there is code.
func TestAMarkerInsideAStringIsCode(t *testing.T) {
	src := "package p\n\nvar s = \"// not a comment\"\n"
	at := strings.Index(src, "// not")
	res := Apply("p.go", src, []edit.Edit{{Start: at, End: at + len("// not a comment"), Text: "// x"}}, edit.Scope{})
	require.Len(t, res.Refused, 1)
	assert.Equal(t, src, res.Text)
}

// A bad edit costs only itself: the good edit beside it still lands.
func TestABadEditDoesNotCostTheGoodOne(t *testing.T) {
	good := commentEdit(t, "p.go", goSrc, "Answer returns", "// Answer returns it.")
	bad := commentEdit(t, "p.go", goSrc, "block", "/* */ var evil = 1")
	res := Apply("p.go", goSrc, []edit.Edit{good, bad}, edit.Scope{})
	assert.Len(t, res.Applied, 1)
	assert.Len(t, res.Refused, 1)
	assert.Contains(t, res.Text, "// Answer returns it.")
	assert.NotContains(t, res.Text, "evil")
}

// An edit outside the scope never lands, and the scope follows its text.
func TestAScopeHoldsEveryEditInside(t *testing.T) {
	first := commentEdit(t, "p.go", goSrc, "Answer returns", "// Answer returns the answer, always.")
	second := commentEdit(t, "p.go", goSrc, "block", "/* a block */")
	res := Apply("p.go", goSrc, []edit.Edit{first, second}, edit.Within(second.Start, second.End))
	assert.Empty(t, res.Refused)
	assert.Len(t, res.Applied, 1)
	assert.Equal(t, "/* a block */", res.Text[res.Scope.Start:res.Scope.End])
	assert.Contains(t, res.Text, "// Answer returns the answer.\n")
}

// The shell grammar reads a hash comment: a newline out of it is code.
func TestAShellCommentCannotEscape(t *testing.T) {
	src := "#!/bin/sh\n# say hello\necho hi\n"
	e := commentEdit(t, "s.sh", src, "say hello", "# say hello\nrm -rf /")
	res := Apply("s.sh", src, []edit.Edit{e}, edit.Scope{})
	require.Len(t, res.Refused, 1)
	assert.Equal(t, src, res.Text)
}

// The interpreter line is read by the kernel, so no repair may write it.
func TestTheInterpreterLineIsNotAComment(t *testing.T) {
	src := "#!/bin/sh\necho hi\n"
	res := Apply("s.sh", src, []edit.Edit{{Start: 0, End: len("#!/bin/sh"), Text: "#!/bin/bash"}}, edit.Scope{})
	require.Len(t, res.Refused, 1)
	assert.Equal(t, src, res.Text)
}
