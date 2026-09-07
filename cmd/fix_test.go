package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
)

func TestACategoryTurnsItsWholeRuleSetOn(t *testing.T) {
	rules, ids, err := selectedRules([]string{"counts", " ste "})
	require.NoError(t, err)

	assert.Equal(t, []slopfix.Rule{slopfix.RuleCounts, slopfix.RuleSTE}, rules)
	assert.Empty(t, ids)
}

// An ID turns its category on too, so naming a rule never needs the category
// named beside it.
func TestARuleIDTurnsItsCategoryOn(t *testing.T) {
	rules, ids, err := selectedRules([]string{"ste/semicolon"})
	require.NoError(t, err)

	assert.Equal(t, []slopfix.Rule{slopfix.RuleSTE}, rules)
	assert.Equal(t, []string{"ste/semicolon"}, ids)
}

// A typo is an error rather than a silent no-op, because a run that applies
// nothing reads as a clean file.
func TestATypoIsAnError(t *testing.T) {
	_, _, err := selectedRules([]string{"ste/nosuch"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ste/semicolon")

	_, _, err = selectedRules([]string{"nosuch"})
	require.Error(t, err)

	_, _, err = selectedRules([]string{"nosuch/rule"})
	require.Error(t, err)
}
