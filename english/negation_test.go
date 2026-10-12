package english

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// cutPattern is a pattern that deletes what it matches.
func cutPattern(match string) Pattern {
	return Pattern{Match: match, re: regexp.MustCompile(match)}
}

// A cut that takes a negation and leaves what it negated says the opposite.
func TestACutNeverDropsANegation(t *testing.T) {
	cut := cutPattern(`(?i)\s*\bno longer\b`)
	for _, line := range []string{
		"prune drops every registration whose descriptor no longer refers to the file registered",
		"The cache is valid no longer.",
	} {
		out, took := cut.ApplyN(line)
		assert.Equal(t, line, out)
		assert.Zero(t, took)
	}
}

// The control: a cut that takes the negation with the whole claim it makes
// leaves no inverted clause behind.
func TestACutMayTakeANegationWithItsClaim(t *testing.T) {
	cut := cutPattern(`(?i)\s*\bdo not reintroduce\b[^.]*`)
	out, took := cut.ApplyN("The cap is off do not reintroduce it. It holds.")
	assert.Equal(t, "The cap is off. It holds.", out)
	assert.Equal(t, 1, took)
}

// A cut beside a negation keeps the negation, so it still lands.
func TestACutBesideANegationLands(t *testing.T) {
	cut := cutPattern(`(?i)\s*\banymore\b`)
	out, _ := cut.ApplyN("The walk does not recurse anymore")
	assert.Equal(t, "The walk does not recurse", out)
}
