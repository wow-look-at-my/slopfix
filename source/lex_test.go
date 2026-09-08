package source

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// texts returns the comment text of every comment the file carries.
func texts(filename, src string) []string {
	var out []string
	for _, c := range Extract(filename, src) {
		out = append(out, c.Text)
	}
	return out
}

// A comment marker inside a literal is data. This is the property the whole
// walk exists to get right.
func TestAMarkerInsideALiteralIsNotAComment(t *testing.T) {
	for name, tc := range map[string]struct {
		file string
		src  string
		want []string
	}{
		"go string":     {"x.go", "const u = \"http://x/y\" // real\n", []string{"// real"}},
		"go raw":        {"x.go", "const u = `a // b`\n// real\n", []string{"// real"}},
		"shell single":  {"x.sh", "echo 'a # b'\n# real\n", []string{"# real"}},
		"python triple": {"x.py", "d = \"\"\"a # b\"\"\"\n# real\n", []string{"# real"}},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, texts(tc.file, tc.src))
		})
	}
}

// Rust nests its block comments, so an inner open has to be counted. Reading
// the earliest close as the end leaves the rest of the file as code.
func TestANestedBlockCommentClosesAtItsOwnEnd(t *testing.T) {
	src := "/* outer /* inner */ still outer */\nlet n = 1;\n"
	got := texts("x.rs", src)
	require.Len(t, got, 1)
	assert.Equal(t, "/* outer /* inner */ still outer */", got[0])
}

// The C family does not nest, so the same text ends at the earliest close. This
// is the control that proves the case above reads the table.
func TestAPlainBlockCommentClosesAtTheFirstEnd(t *testing.T) {
	got := texts("x.c", "/* outer /* inner */ still outer */\nint n = 1;\n")
	require.NotEmpty(t, got)
	assert.Equal(t, "/* outer /* inner */", got[0])
}

// A raw literal carries its own hashes in the close, and a marker inside it is
// data whatever it looks like.
func TestARustRawLiteralIsNotProse(t *testing.T) {
	src := "let s = r#\"a \" b // c\"#;\n// real\n"
	assert.Equal(t, []string{"// real"}, texts("x.rs", src))
}

// A lifetime is spelled with the character a literal opens with, and it has no
// close. Reading it as a literal swallows the rest of the line.
func TestALifetimeIsNotACharacterLiteral(t *testing.T) {
	src := "fn f<'a>(x: &'a str) { /* real */ }\n"
	assert.Equal(t, []string{"/* real */"}, texts("x.rs", src))
}

// A heredoc body is input to the command, so a marker in it is data.
func TestAHeredocBodyIsNotProse(t *testing.T) {
	src := strings.Join([]string{
		"cat <<EOF",
		"# not a comment",
		"EOF",
		"# real",
	}, "\n")
	assert.Equal(t, []string{"# real"}, texts("x.sh", src))
}

// A quoted heredoc delimiter behaves the same way.
func TestAQuotedHeredocDelimiterIsRead(t *testing.T) {
	src := strings.Join([]string{
		"cat <<'EOF'",
		"# not a comment",
		"EOF",
		"# real",
	}, "\n")
	assert.Equal(t, []string{"# real"}, texts("x.sh", src))
}

// A shell passes `a#b` to the command as a single word, so the marker opens a
// comment only where a word begins.
func TestAHashInsideAWordIsNotAComment(t *testing.T) {
	assert.Empty(t, texts("x.sh", "echo a#b\n"))
	assert.Equal(t, []string{"# real"}, texts("x.sh", "echo a # real\n"))
}

// Every byte belongs to a single span, in order. A gap loses text and an
// overlap reports the same byte again.
func TestTheSpansCoverEveryByteInOrder(t *testing.T) {
	src := "package p\n\n// note\nconst u = \"x\" // trailing\n"
	spans, ok := Lex("x.go", src)
	require.True(t, ok)
	require.NotEmpty(t, spans)

	at := 0
	for _, s := range spans {
		assert.Equal(t, at, s.Start, "a span starts where the last one ended")
		assert.Greater(t, s.End, s.Start, "a span holds at least one byte")
		at = s.End
	}
	assert.Equal(t, len(src), at, "the spans reach the end of the file")
}

// A language the table does not carry is skipped rather than guessed at.
func TestAnUnknownLanguageIsNotRead(t *testing.T) {
	assert.False(t, Supported("x.unknownext"))
	_, ok := Lex("x.unknownext", "// looks like a comment")
	assert.False(t, ok)
	assert.Empty(t, Extract("x.unknownext", "// looks like a comment"))
}

// A file the build reads by name rather than by extension is carried too.
func TestAFileNamedRatherThanSuffixedIsRead(t *testing.T) {
	for _, name := range []string{"Makefile", "Dockerfile", "justfile", "Dockerfile.ci"} {
		assert.True(t, Supported(name), "%s", name)
		assert.Equal(t, []string{"# real"}, texts(name, "# real\n"), "%s", name)
	}
}

// An unterminated literal consumes the rest, which stops a stray quote from
// turning the remaining file into prose.
func TestAnUnterminatedLiteralSwallowsTheRest(t *testing.T) {
	assert.Empty(t, texts("x.go", "const u = \"unterminated\n// not prose\n"))
}
