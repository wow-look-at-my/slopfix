package slopfix_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// autofixPromise is the phrase a finding's remedy uses to tell the reader that `slopfix fix` repairs it.
const autofixPromise = "`slopfix fix` does this."

// materialize writes a tree case's files under a fresh directory with the
// repository marker the tree rules read. A file case is returned unchanged.
func materialize(t *testing.T, c slopfix.RuleCase) slopfix.RuleCase {
	t.Helper()
	if c.Files == nil {
		return c
	}
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	for name, content := range c.Files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	c.Root = root
	return c
}

// TestEveryRuleAutofixClearsItsOwnDetection is the harness. For every
// registered rule: each of its cases fires its detection; the autofix runs;
// the detection is silent the second time. A rule whose autofix does not clear
// its own detection fails the build here, rather than shipping a detection
// with no way out of it.
func TestEveryRuleAutofixClearsItsOwnDetection(t *testing.T) {
	for _, rule := range slopfix.AllRuleSpecs() {
		rule := rule
		t.Run(rule.ID, func(t *testing.T) {
			require.NotEmpty(t, rule.Cases, "%s carries no case", rule.ID)
			for _, raw := range rule.Cases {
				c := materialize(t, raw)
				before := rule.Detect(c)
				require.NotEmpty(t, before, "%s: case %q does not fire its detection", rule.ID, c.Name)
				for _, f := range before {
					assert.Equal(t, rule.ID, f.ID, "%s: case %q reported another rule", rule.ID, c.Name)
				}
				if rule.Autofix == nil {
					// The declared exemption. No rewrite answers this rule, so the harness checks only that its detection is real.
					assert.NotEmpty(t, rule.ReportOnly, "%s: no autofix and no declared exemption", rule.ID)
					continue
				}
				after := rule.Detect(rule.Autofix(c))
				assert.Empty(t, after, "%s: case %q still detects after its own autofix", rule.ID, c.Name)
			}
		})
	}
}

// A rule whose finding tells the reader `slopfix fix` does this owes a working
// autofix. A message that advertises a repair the rule does not have, or an
// autofix that fires on nothing, fails here rather than shipping a promise.
// The tool cannot keep.
func TestAMessagePromisingAnAutofixCarriesOne(t *testing.T) {
	for _, rule := range slopfix.AllRuleSpecs() {
		rule := rule
		t.Run(rule.ID, func(t *testing.T) {
			promises := false
			for _, raw := range rule.Cases {
				c := materialize(t, raw)
				for _, f := range rule.Detect(c) {
					if strings.Contains(f.Fix, autofixPromise) {
						promises = true
					}
				}
			}
			if !promises {
				return
			}
			require.NotNil(t, rule.Autofix, "%s: a finding promises an autofix and the rule carries none", rule.ID)
			assert.Empty(t, rule.ReportOnly, "%s: it promises an autofix and declares report-only", rule.ID)
			for _, raw := range rule.Cases {
				c := materialize(t, raw)
				after := rule.Autofix(c)
				assert.Empty(t, rule.Detect(after), "%s: case %q still detects after the promised autofix", rule.ID, c.Name)
			}
		})
	}
}

// A rule with a detection needs an autofix or a declared exemption. The
// registry refuses the rest at init, so an unfixed detection fails to register.
func TestRegistrationRefusesADetectionWithoutAnAutofix(t *testing.T) {
	assert.Panics(t, func() {
		slopfix.RegisterRule(slopfix.RuleSpec{
			ID:       "probe/no-autofix",
			Category: slopfix.RuleSTE,
			Detect:   func(slopfix.RuleCase) []ste.Finding { return nil },
			Cases:    []slopfix.RuleCase{{Text: "x"}},
		})
	})
}

// Every rule a category claims is registered, so --only never selects an empty
// set and reads as a clean file.
func TestEveryCategoryRuleIsRegistered(t *testing.T) {
	for _, category := range slopfix.AllRules {
		for id := range slopfix.IDsFor(category).All() {
			_, ok := slopfix.RuleSpecByID(id)
			assert.True(t, ok, "%s is in category %s and is not registered", id, category)
		}
	}
}

// The report-only and warning rules are enumerated one by one, so a new rule
// cannot join their ranks in passing.
func TestOnlyTheseRulesReportWithoutAnAutofix(t *testing.T) {
	exempt := set.New[string]()
	for _, rule := range slopfix.AllRuleSpecs() {
		if rule.Autofix == nil {
			exempt.Add(rule.ID)
			assert.NotEmpty(t, rule.ReportOnly, "%s: an exemption states no reason", rule.ID)
			continue
		}
		assert.Empty(t, rule.ReportOnly, "%s repairs, so it cannot claim an exemption", rule.ID)
	}
	for id := range slopfix.WarningIDs.All() {
		assert.True(t, exempt.Contains(id), "warning %s is not declared report-only", id)
	}
	for _, id := range []string{"laziness/punt", "blame/deflection", "ask/prose-decision", "yaml/test-in-workflow"} {
		assert.True(t, exempt.Contains(id), "%s is not declared report-only", id)
		spec, ok := slopfix.RuleSpecByID(id)
		require.True(t, ok, "%s is not registered", id)
		assert.NotEmpty(t, spec.ReportOnly, "%s carries no declared exemption", id)
		assert.True(t, spec.Autofix == nil, "%s declares an exemption and an autofix", id)
	}
	assert.False(t, slopfix.Repairable("laziness/punt"))
}
