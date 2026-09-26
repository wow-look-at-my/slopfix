package slopfix_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/fixer"
)

// Every rule the binary claims to repair has a registered fixer behind it. A
// repair that lives outside the registry writes outside the gates.
func TestEveryRepairableRuleHasARegisteredFixer(t *testing.T) {
	served := set.New[string]()
	for _, fx := range fixer.All() {
		for _, id := range fx.IDs() {
			served.Add(id)
		}
	}
	for id := range slopfix.AllIDs().All() {
		if slopfix.Repairable(id) {
			assert.True(t, served.Contains(id), "%s is repairable and no registered fixer serves it", id)
		}
	}
}

// The registry holds every fixer in the order each kind runs them.
func TestTheRegistryRunsEachKindInOrder(t *testing.T) {
	names := func(kind fixer.Kind) []string {
		var out []string
		for _, fx := range fixer.For(kind) {
			out = append(out, fx.Name())
		}
		return out
	}
	assert.Equal(t, []string{"tombstones", "comments/length", "comments/number", "comments/length-after-number"}, names(fixer.Source))
	assert.Equal(t, []string{"tombstones", "counts/inventory-count", "wrap-and-ste", "ste/count"}, names(fixer.Document))
	assert.Equal(t, []string{"yaml/ungate", "yaml/join-comments", "yaml/rename-guarded-job", "yaml/untest"}, names(fixer.Workflow))
}

// Every family a fixer answers to is one --only accepts.
func TestEveryFixerFamilyIsARule(t *testing.T) {
	for _, fx := range fixer.All() {
		require.NotEmpty(t, fx.Categories(), fx.Name())
		for _, family := range fx.Categories() {
			assert.True(t, slices.Contains(slopfix.AllRules, slopfix.Rule(family)), "%s answers to %q, which --only does not accept", fx.Name(), family)
		}
	}
}

// A name taken twice is a programming error, caught at init.
func TestRegisteringANameTwicePanics(t *testing.T) {
	taken := fixer.All()[0]
	assert.Panics(t, func() {
		fixer.Register(fixer.Spec{Label: taken.Name(), Repair: func(*fixer.File) {}})
	})
}
