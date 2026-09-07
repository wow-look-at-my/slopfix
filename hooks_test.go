package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
)

// The assertion the table exists for. A rule added with no home fails here, and
// so does an entry naming a rule somebody deleted. Without it the mapping lives
// in nobody's head and drifts in silence.
func TestEveryRuleHasAHomeAndEveryHomeNamesARealRule(t *testing.T) {
	every := slopfix.EveryRuleID()
	require.False(t, every.IsEmpty())

	homed := set.New[string]()
	for _, hook := range slopfix.Hooks() {
		ids := slopfix.Selects(hook)
		if hook.Pending {
			assert.True(t, ids.IsEmpty(), "%s selects a rule slopfix does not hold yet", hook.Name)
			continue
		}
		assert.False(t, ids.IsEmpty(), "%s selects nothing at all", hook.Name)
		for id := range ids.All() {
			assert.True(t, every.Contains(id), "%s names %q, which no rule reports", hook.Name, id)
			homed.Add(id)
		}
	}

	for id := range every.All() {
		assert.True(t, homed.Contains(id), "%q has no home in the hook table", id)
	}
}

// The negative control. A rule really added to the universe is really unhomed,
// so the case above passes on coverage rather than on an empty walk.
func TestAnUnhomedRuleIsCaught(t *testing.T) {
	homed := set.New[string]()
	for _, hook := range slopfix.Hooks() {
		homed = homed.Union(slopfix.Selects(hook))
	}
	assert.False(t, homed.Contains("ste/not-a-real-rule"))
}

// A hook that is a named selection and nothing else. Reading the table by name
// is what a caller does, so a missing name must not answer with a usable zero.
func TestAHookIsFoundByName(t *testing.T) {
	hook, ok := slopfix.FindHook("no-counts-in-docs")
	require.True(t, ok)
	assert.Equal(t, "PreToolUse", hook.Event)
	assert.True(t, slopfix.Selects(hook).Contains(slopfix.IDInventoryCount))

	_, ok = slopfix.FindHook("nosuch")
	assert.False(t, ok)
}

// The default set is what common-checks means, per the operator's ruling. It
// selects no rule by name, and resolves to what the check path reports.
func TestTheDefaultSetIsTheWholeCheckPath(t *testing.T) {
	hook, ok := slopfix.FindHook("common-checks")
	require.True(t, ok)
	assert.Empty(t, hook.Only)
	assert.True(t, slopfix.Selects(hook).Equal(slopfix.AllIDs()))
}
