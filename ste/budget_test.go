package ste_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
)

// A run over its budget is tightened, and every clause it held stays.
func TestTheBudgetTightensAndKeepsEveryClause(t *testing.T) {
	text := "It actually reads the value and it actually writes the result."
	tighten := func(s string) string { return strings.ReplaceAll(s, " actually", "") }
	out, fit := ste.FitToBudget(text, ste.BudgetChars(text)-1, tighten)
	assert.True(t, fit, "the tightened run fits")
	assert.Equal(t, "It reads the value and it writes the result.", out)
}

// A run the tightening cannot shorten is reported, not emptied.
func TestTheBudgetLeavesAnUnshortenableRunWhole(t *testing.T) {
	text := "The loader reads the value and the caller waits for the result."
	out, fit := ste.FitToBudget(text, ste.BudgetChars(text)-1, nil)
	assert.False(t, fit, "no pass shortens it")
	assert.Equal(t, text, out, "nothing was dropped")
}
