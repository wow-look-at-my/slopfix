package cardinal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/cardinal"
)

// A measurement is a value in a unit, never a tally of items. No set changes
// underneath it, and every rewrite of it writes a false sentence: "a set value
// fps", "many ms". No substrate reports one.
func TestAMeasurementIsNoCount(t *testing.T) {
	for _, prose := range []string{
		"it waits three seconds before the next poll",
		"the header is eight bytes wide",
		"the frame budget holds at 60 fps",
		"the raster takes 49 ms on this scene",
		"the permutations sit at 48 VGPRs",
		"this may drop them to 4 waves/SIMD",
	} {
		assert.Empty(t, cardinal.Find(prose, cardinal.Gate), "gate: %s", prose)
		assert.Empty(t, cardinal.Find(prose, cardinal.Comment), "comment: %s", prose)
	}
}

// A value in a comment is no count. This covers a flag's value, a member of a
// set literal, one side of an assignment, a measurement, or the subject of
// "is".
func TestACommentValueIsNoCount(t *testing.T) {
	for _, prose := range []string{
		"which is what makes --warp-field 0 the pre-field warp.",
		"(N in {1,2,4,8}; 4 is the default, 0 = off)",
		"still produce mask exactly 0.",
		"the shipped per-texel field has no 16 px cell structure",
	} {
		assert.Empty(t, cardinal.Find(prose, cardinal.Comment), "comment: %s", prose)
	}
}

// The control. A cardinal governing items the reader can go and count is what
// the rule is for, and it still fires.
func TestATallyOfItemsIsStillACount(t *testing.T) {
	for _, prose := range []string{
		"This repo's 15 plugins ride in the payload.",
		"It ships two hooks.",
		"There are three sections.",
	} {
		assert.NotEmpty(t, cardinal.Find(prose, cardinal.Gate), "gate: %s", prose)
	}
}

func TestIsUnitReadsTheClassFromTheRulesFolder(t *testing.T) {
	assert.True(t, cardinal.IsUnit("seconds"))
	assert.True(t, cardinal.IsUnit("Bytes"))
	assert.True(t, cardinal.IsUnit("characters."))
	assert.False(t, cardinal.IsUnit("plugins"))
	assert.False(t, cardinal.IsUnit("hooks"))
}
