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
		"not XML":       `<laziness answer="a">`,
		"no answer":     `<laziness><shape name="s"><pattern re="x"><punt>x</punt></pattern></shape></laziness>`,
		"no pattern":    `<laziness answer="a"></laziness>`,
		"bad regexp":    `<laziness answer="a"><shape name="s"><pattern re="("><punt>x</punt></pattern></shape></laziness>`,
		"no punt":       `<laziness answer="a"><shape name="s"><pattern re="x"/></shape></laziness>`,
		"no shape name": `<laziness answer="a"><shape><pattern re="x"><punt>x</punt></pattern></shape></laziness>`,
		"bad pardon":    `<laziness answer="a"><shape name="s"><pattern re="x"><punt>x</punt></pattern></shape><pardon re="("/></laziness>`,
	} {
		_, err := laziness.Load([]byte(raw))
		assert.Error(t, err, name)
	}
}

// The control for the cases above: the smallest well-formed table loads.
func TestTheSmallestTableLoads(t *testing.T) {
	got, err := laziness.Load([]byte(`<laziness answer="a"><shape name="s"><pattern re="x"><punt>x</punt></pattern></shape></laziness>`))
	require.NoError(t, err)
	assert.Equal(t, "a", got.Answer)
	assert.Len(t, got.Patterns, 1)
}

// The Stop hook approves with the word the table names.
func TestTheAnswerIsContinue(t *testing.T) {
	assert.Equal(t, "continue", laziness.Rules.Answer)
}
