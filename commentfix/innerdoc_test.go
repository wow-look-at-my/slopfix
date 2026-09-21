package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.False(t, strings.HasPrefix(hit.Sentence, "!"),
			"the marker reached the prose: %s", hit.Sentence)
	}
}

func TestTheRepairNeverSplitsTheInnerDocMarker(t *testing.T) {
	fixed, _ := FixLength("x.rs", innerDoc)
	assert.NotContains(t, fixed, "// !", "the repair split the marker")
	assert.NotContains(t, fixed, ".!", "the repair welded a marker onto the prose")
}
