package commentlength

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spans reports what the scanner decided, so a case asserts the measurement.
// A wrong span deletes the wrong prose.
func spans(t *testing.T, src string) []block {
	t.Helper()
	return blocks("x.go", src)
}

// The span is the declaration's own, not everything below it. A comment above
// a short function is weighed against that function and nothing after it.
func TestTheSpanIsTheDeclarationItDocuments(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// Serve wires the routes.",
		"func Serve() {",
		"\tprintln(1)",
		"}",
		"",
		"func Unrelated() {",
		"\tprintln(2)",
		"\tprintln(3)",
		"\tprintln(4)",
		"}",
	}, "\n")

	var doc block
	for _, b := range spans(t, src) {
		if strings.Contains(b.text[0], "Serve wires") {
			doc = b
		}
	}
	require.NotZero(t, doc.codeLines, "the comment was not paired with a declaration")
	assert.Equal(t, 3, doc.codeLines, "func, body and closing brace, and nothing after them")
}

// A blank line inside a function body does not end the span. The parser knows
// where the body ends; a line walk has to guess, and guesses short.
func TestABlankLineInsideABodyDoesNotEndTheSpan(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// Run does the thing.",
		"func Run() {",
		"\tprintln(1)",
		"",
		"\tprintln(2)",
		"}",
	}, "\n")

	bs := spans(t, src)
	require.NotEmpty(t, bs)
	var doc block
	for _, b := range bs {
		if strings.Contains(b.text[0], "Run does") {
			doc = b
		}
	}
	assert.Equal(t, 4, doc.codeLines, "the blank line is part of the body")
}

// A file that does not parse yields nothing. Half a syntax tree is a worse
// input than none, and a file mid-edit is the common case for a hook.
func TestAnUnparseableGoFileYieldsNothing(t *testing.T) {
	src := "package p\n\n// A comment that would otherwise be measured.\nfunc Broken( {\n"
	assert.Empty(t, spans(t, src))
	assert.Empty(t, Check("x.go", src))

	out, changed := Fix("x.go", src)
	assert.False(t, changed)
	assert.Equal(t, src, out)
}

// A package comment is weighed against its own clause, not against the whole
// file. Otherwise no package comment could ever be too long.
func TestAPackageCommentIsWeighedAgainstItsClause(t *testing.T) {
	src := strings.Join([]string{
		"// Package p exists for a reason that takes several lines to set out, and",
		"// goes on setting it out well past the length of anything it could",
		"// reasonably be measured against if the whole file counted as its code.",
		"package p",
		"",
		"func A() { println(1) }",
		"func B() { println(2) }",
		"func C() { println(3) }",
	}, "\n")

	hits := Check("x.go", src)
	require.NotEmpty(t, hits, "a package comment is measured against its clause")
	assert.Equal(t, 1, hits[0].Line)
}

// A doc comment on a struct field is a block of its own, so a long note on a
// single-line field is found rather than folded into the type.
func TestAFieldDocIsItsOwnBlock(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"type T struct {",
		"\t// Name is the thing this is called, which takes rather more words to",
		"\t// explain than the field itself takes to declare, by a wide margin.",
		"\tName string",
		"}",
	}, "\n")

	hits := Check("x.go", src)
	require.Len(t, hits, 1)
	assert.Equal(t, 4, hits[0].Line)
}

// A comment beside a statement is not a doc comment, so it is never measured.
func TestAFreeFloatingCommentIsNotMeasured(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"func Run() {",
		"\t// This note sits inside a body and documents no declaration at all,",
		"\t// so there is nothing to weigh it against and it is left alone.",
		"\tprintln(1)",
		"}",
	}, "\n")
	assert.Empty(t, Check("x.go", src))
}

// The repair must not corrupt the file: what it writes still parses, and the
// declaration it documented is untouched.
func TestTheRepairLeavesTheFileParseable(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"// Base is the host.",
		"//",
		"// It used to be derived here, which is how every published link kept the",
		"// legacy spelling long after the canonical form moved, and the fallback",
		"// below is for a server older than the field that reports it.",
		`const Base = "https://example.invalid"`,
	}, "\n")

	out, changed := Fix("x.go", src)
	require.True(t, changed)
	assert.Contains(t, out, "// Base is the host.")
	assert.Contains(t, out, `const Base = "https://example.invalid"`)
	assert.NotContains(t, out, "legacy spelling")

	_, ok := goBlocks(out)
	assert.True(t, ok, "the repaired file still parses")
	assert.Empty(t, Check("x.go", out), "the repaired file is clean")
}
