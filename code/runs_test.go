package code

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheDocCommentsOfAdjacentFieldsAreSeparateRuns(t *testing.T) {
	src := `pub struct Tracker {
    /// Entry that receives message deltas.
    /// None between turns.
    current: Option<u32>,
    /// Entry that receives thought deltas.
    thinking: Option<u32>,
}
`
	runs, ok := Runs("tracker.rs", src)
	require.True(t, ok)
	require.Len(t, runs, 2, "a field line between doc comments ends the run")
	assert.Equal(t, 1, runs[0].Start)
	assert.Equal(t, 3, runs[0].End)
	assert.Equal(t, []bool{true, true}, runs[0].Pure)
	assert.Equal(t, 4, runs[1].Start)
	assert.Equal(t, 5, runs[1].End)
}

func TestAdjoiningRustLineCommentsStayOneRun(t *testing.T) {
	src := "// one\n// two\n// three\nfn f() {}\n"
	runs, ok := Runs("lib.rs", src)
	require.True(t, ok)
	require.Len(t, runs, 1)
	assert.Equal(t, 0, runs[0].Start)
	assert.Equal(t, 3, runs[0].End)
}
