package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every line here is prose the repair rewrote in a shipped repository, where
// "a single" landed in a slot an article already held. Each reads as English
// again only because the cardinal stays put.
func TestAPronounKeepsTheCardinal(t *testing.T) {
	for _, prose := range []string{
		"and the wrong one when there is not.",
		"A smaller one keeps the fraction.",
		"be tested without one that is actually full.",
		"A daemon that says nothing then reads as one keeping up.",
		"reports every one still under its target.",
	} {
		assert.Equal(t, prose, Say(prose))
	}
}

// The determiner still rewrites. Refusing the pronoun must not cost the case
// the repair exists for.
func TestADeterminerStillRewrites(t *testing.T) {
	assert.Equal(t, "It reserves a single slot", Say("It reserves one slot"))
	assert.Equal(t, "what should happen to a single path handed to it", Say("what should happen to one path handed to it"))
	assert.Equal(t, "A single slot is reserved", Say("One slot is reserved"))
}

// A phrase entry in the table reads the cardinal before this does, so the
// idioms it covers keep their own repair.
func TestAPhraseEntryStillWins(t *testing.T) {
	assert.Equal(t, "It owns the thing below", Say("It owns the one below"))
	assert.Equal(t, "It is any of the keys", Say("It is one of the keys"))
	assert.Equal(t, "It reads each", Say("It reads each one"))
}

// A word merely holding the cardinal is a name.
func TestANameHoldingTheCardinalIsLeftAlone(t *testing.T) {
	assert.Equal(t, "The oneShot flag and someone else", Say("The oneShot flag and someone else"))
	assert.Equal(t, "It calls Do.One here", Say("It calls Do.One here"))
}

// The cardinal with nothing after it names no noun, so it stays.
func TestACardinalEndingTheLineStays(t *testing.T) {
	assert.Equal(t, "It keeps one", Say("It keeps one"))
	assert.Equal(t, "It keeps one.", Say("It keeps one."))
}
