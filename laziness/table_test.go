package laziness_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/laziness"
)

// Every pattern must fire on each punt under it. A row nothing can fire
// reads as coverage.
func TestEveryPatternFiresOnItsPunts(t *testing.T) {
	for _, p := range laziness.Rules.Patterns {
		for _, punt := range p.Punts {
			assert.True(t, p.Re.MatchString(punt), "%q does not match its punt %q", p.Re, punt)
			hits := laziness.Check(punt)
			if assert.NotEmpty(t, hits, "no finding on %q", punt) {
				assert.Equal(t, laziness.ID, hits[0].ID)
			}
		}
	}
}

// Each pardon must match its examples, and the example must come out clean.
func TestEveryPardonClearsItsExamples(t *testing.T) {
	require.NotEmpty(t, laziness.Rules.Pardons)
	for _, p := range laziness.Rules.Pardons {
		for _, clean := range p.Clean {
			assert.True(t, p.Re.MatchString(clean), "%q does not match %q", p.Re, clean)
			assert.Empty(t, laziness.Check(clean), "false finding on %q", clean)
		}
	}
}

// A pardon example proves nothing unless a pattern would fire without it.
func TestEveryPardonExampleCarriesAPattern(t *testing.T) {
	for _, p := range laziness.Rules.Pardons {
		for _, clean := range p.Clean {
			fired := false
			for _, q := range laziness.Rules.Patterns {
				fired = fired || q.Re.MatchString(clean)
			}
			assert.True(t, fired, "no pattern fires on pardon example %q", clean)
		}
	}
}

// The negative controls. A message that owns the work carries no finding.
func TestTheCleanMessagesFireNothing(t *testing.T) {
	require.NotEmpty(t, laziness.Rules.Clean)
	for _, message := range laziness.Rules.Clean {
		assert.Empty(t, laziness.Check(message), "false finding on %q", message)
	}
}

func TestAMalformedTableIsAnError(t *testing.T) {
	for name, raw := range map[string]string{
		"not XML":       `<laziness>`,
		"no pattern":    `<laziness></laziness>`,
		"bad regexp":    `<laziness><shape name="s"><pattern re="("><punt>x</punt></pattern></shape></laziness>`,
		"no punt":       `<laziness><shape name="s"><pattern re="x"/></shape></laziness>`,
		"no shape name": `<laziness><shape><pattern re="x"><punt>x</punt></pattern></shape></laziness>`,
		"bad pardon":    `<laziness><shape name="s"><pattern re="x"><punt>x</punt></pattern></shape><pardon re="("/></laziness>`,
	} {
		_, err := laziness.Load([]byte(raw))
		assert.Error(t, err, name)
	}
}
