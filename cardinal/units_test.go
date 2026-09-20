package cardinal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/cardinal"
)

// The sentences a repair destroyed.
func TestAMeasurementIsNotACount(t *testing.T) {
	for _, prose := range []string{
		"a `list` that walks each item's tree walks the whole bin every 30 seconds",
		"the daemon gives back the oldest items after two minutes",
		"a fixed 260-character path reads 260 characters",
		"it waits three seconds before the next poll",
		"the header is eight bytes wide",
		"it reports 12 lines of context",
	} {
		assert.Empty(t, cardinal.Find(prose, cardinal.Gate), "gate: %s", prose)
		assert.Empty(t, cardinal.Find(prose, cardinal.Prose), "prose: %s", prose)
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
