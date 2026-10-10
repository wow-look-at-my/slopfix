package edit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noBad is a gate that refuses any text holding BAD, and counts its calls.
func noBad(calls *int) func(string) bool {
	return func(text string) bool {
		*calls++
		return !strings.Contains(text, "BAD")
	}
}

// One bad edit among many lands every good one, and the gate parses a few
// times rather than once for each edit.
func TestGateRefusesOnlyTheBadEditAndParsesFewTimes(t *testing.T) {
	src := strings.Repeat("x\n", 256)
	var edits []Edit
	for i := range 256 {
		text := "y"
		if i == 100 {
			text = "BAD"
		}
		edits = append(edits, Edit{Start: 2 * i, End: 2*i + 1, Text: text})
	}
	calls := 0
	res := Gate(src, edits, Scope{}, func(Edit) string { return "" }, noBad(&calls))
	require.Len(t, res.Refused, 1)
	assert.Equal(t, 200, res.Refused[0].Edit.Start)
	assert.Len(t, res.Applied, 255)
	assert.Equal(t, 255, strings.Count(res.Text, "y"))
	assert.NotContains(t, res.Text, "BAD")
	assert.Less(t, calls, 40, "a bisection costs a few parses for each bad edit")
}

func TestGateRefusesEveryBadEdit(t *testing.T) {
	src := "a\nb\nc\nd\n"
	edits := []Edit{{Start: 0, End: 1, Text: "BAD"}, {Start: 2, End: 3, Text: "B"}, {Start: 4, End: 5, Text: "BAD"}, {Start: 6, End: 7, Text: "D"}}
	calls := 0
	res := Gate(src, edits, Scope{}, func(Edit) string { return "" }, noBad(&calls))
	assert.Equal(t, "a\nB\nc\nD\n", res.Text)
	assert.Len(t, res.Refused, 2)
}

// The line cache keys a text by its bytes, so texts of one length never share an answer.
func TestLinesReadsEachTextOnItsOwn(t *testing.T) {
	first := "ab\ncd\nef"
	second := "abcd\ne\nf"
	assert.Equal(t, Edit{Start: 3, End: 5, Text: "X"}, Lines(first, 1, 1, 0, []string{"X"}))
	assert.Equal(t, Edit{Start: 5, End: 6, Text: "X"}, Lines(second, 1, 1, 0, []string{"X"}))
	assert.Equal(t, Edit{Start: 3, End: 5, Text: "X"}, Lines(first, 1, 1, 0, []string{"X"}))
}
