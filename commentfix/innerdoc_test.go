package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rust spells an inner doc comment `//!`. Read as `//` it leaves a `!` at the head
// of the prose, so the finding quotes a sentence the author never wrote and the
// repair emits `// !` with the collected marks welded onto the tail.
const innerDoc = "//! An explanation that runs well past the declaration below it.\n" +
	"//!\n" +
	"//! It carries a second paragraph, so the block is longer than the code.\n" +
	"//! It carries a third, which puts it past anything one declaration holds.\n" +
	"//! It carries a fourth, to leave the repair no doubt about the budget.\n" +
	"//! It carries a fifth, which is where a reader stops reading.\n" +
	"const P: i32 = 1;\n"

func TestAnInnerDocMarkerIsNotProse(t *testing.T) {
	for _, hit := range CheckLength("x.rs", innerDoc) {
		assert.NotContains(t, hit.Message, "! ", "the marker reached the prose: %s", hit.Message)
	}
}

func TestTheRepairKeepsTheInnerDocMarker(t *testing.T) {
	fixed, changed := FixLength("x.rs", innerDoc)
	require.True(t, changed, "the repair did nothing")

	assert.NotContains(t, fixed, "// !", "the repair split the marker")
	assert.NotContains(t, fixed, ".!", "the repair welded a marker onto the prose")
	assert.Contains(t, fixed, "//! An explanation that runs well past", "the opening did not survive")
	assert.Contains(t, fixed, "const P: i32 = 1;", "the repair took the code with it")
	assert.Empty(t, CheckLength("x.rs", fixed), "still over after the repair")
}
