package cardinal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix/cardinal"
)

// A measurement is reported like any other stated count, because the number
// goes stale the same way. What it must never earn is a rewrite that keeps the
// unit and drops the value, which is the rule the class feeds.
func TestAMeasurementIsStillReported(t *testing.T) {
	for _, prose := range []string{
		"it waits three seconds before the next poll",
		"the header is eight bytes wide",
	} {
		assert.NotEmpty(t, cardinal.Find(prose, cardinal.Gate), "gate: %s", prose)
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
