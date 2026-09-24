package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Each pair is a rewrite a run over gh-wait-ci shipped. The guard must refuse
// each of them.
func TestTheGuardRefusesTheShippedWreckage(t *testing.T) {
	for _, c := range []struct{ before, after, defect string }{
		{"The one thing that matters most", "the thing thing that matters most", "the same word twice in a row"},
		{"A zero deadline never expires", "A empty deadline never expires", `"a" before a vowel sound`},
		{"It names the three shapes GitHub uses", "It names each shapes GitHub uses", `"each", "every" or "a single" before a plural noun`},
		{"The walk has two phases", "the walk has phases", "a lowercase start where the original was capitalized"},
	} {
		assert.Equal(t, c.defect, defectOf(c.before, c.after), c.after)
	}
}

// The guard judges what a rewrite ADDS. A defect the author wrote is theirs,
// and refusing a rewrite over it would block every repair of that line.
func TestTheGuardIgnoresADefectTheSourceAlreadyHad(t *testing.T) {
	assert.Empty(t, defectOf("It is is a typo in two places", "It is is a typo in places"))
	assert.Empty(t, defectOf("It names the shapes", "It names the shapes GitHub uses"))
}

// The vowel check reads the sound, not the letter.
func TestTheGuardReadsTheVowelSound(t *testing.T) {
	for _, ok := range []string{"a unit", "a user", "a one-off", "a union", "a European"} {
		assert.Empty(t, defectOf("x", ok), ok)
	}
	for _, bad := range []string{"a empty", "a hour", "a index", "a unset value"} {
		assert.NotEmpty(t, defectOf("x", bad), bad)
	}
}

// A refused step leaves the prose as the step found it, and Fix names the
// file, the line and the entry.
func TestARefusedStepIsReportedWhereItHappened(t *testing.T) {
	got, rejected := rewordRun("The two both agree")
	assert.NotContains(t, got, "Both both", "the-two would write it")
	assert.Len(t, rejected, 1)

	repair := Fix("x.go", "package p\n\n// The two both agree.\nvar x int\n")
	if assert.Len(t, repair.Rejected, 1) {
		r := repair.Rejected[0]
		assert.Equal(t, "x.go", r.Path)
		assert.Equal(t, 3, r.Line)
		assert.Equal(t, "the-two", r.Rule)
		assert.Equal(t, "the same word twice in a row", r.Defect)
		assert.Equal(t, `x.go:3: [comments/number/the-two] discarded a rewrite with the same word twice in a row: "Both both agree"`, r.String())
	}
}
