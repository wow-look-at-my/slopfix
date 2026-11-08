package forkscope

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOffsetMovesWithKeptText(t *testing.T) {
	before := "one\ntwo\nthree\n"
	after := "zero\none\ntwo\nthree\n"

	assert.Equal(t, 5, Offset(before, after, 0, false))
	assert.Equal(t, 9, Offset(before, after, 4, false))
	assert.Equal(t, len(after), Offset(before, after, len(before), true))
}

// A line cut above a span moves the start of the span as much as the end.
func TestOffsetOfASpanAfterACutLine(t *testing.T) {
	before := "header\n\nuse a;\nuse b;\n"
	after := "use a;\nuse b;\n"
	start := len("header\n\n")

	assert.Equal(t, 0, Offset(before, after, start, false))
	assert.Equal(t, len(after), Offset(before, after, len(before), true))
}

// An insertion right on the start of a span stays outside it, and one right on the end stays inside.
func TestOffsetKeepsAnInsertionOnTheEdgeOutsideTheStart(t *testing.T) {
	before := "use a;\nuse b;\n"
	after := "// head\n\nuse a;\nuse b;\n"

	assert.Equal(t, len("// head\n\n"), Offset(before, after, 0, false))
	assert.Equal(t, len(after), Offset(before, after, len(before), true))
	assert.Equal(t, len("// head\n\n"), Offset(before, after, 0, true))
}

// A byte inside replaced text lands on the edge of what replaced it.
func TestOffsetInsideReplacedText(t *testing.T) {
	before := "a\nold one\nold two\nz\n"
	after := "a\nnew\nz\n"

	assert.Equal(t, 2, Offset(before, after, 4, false))
	assert.Equal(t, 6, Offset(before, after, 4, true))
}
