package slopfix_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// everyOtherRule answers what every registered rule except id detects in c.
func everyOtherRule(c slopfix.RuleCase, id string) []string {
	var found []string
	for _, rule := range slopfix.AllRuleSpecs() {
		if rule.ID == id {
			continue
		}
		for _, f := range rule.Detect(c) {
			found = append(found, rule.ID+": "+f.String())
		}
	}
	return found
}

// TestEveryRuleUndergoesTheCommonTest is the test every rule undergoes, on
// every case it carries, in exactly these steps:
//
//   - <input fixture, which includes the case(s) that the rule should detect>
//   - run ALL OTHER rules on the fixture, confirming that there are no detections other than the current rule we're testing
//   - run the rule on the fixture, confirming it is detected
//   - run the rule on the result, confirming it was fixed (i.e. no longer detected)
//   - run ALL OTHER rules on the "fixed" fixture, confirming there are STILL no detctions.
//
// The result is what the rule's autofix writes. No rule and no case has a way
// to skip a step.
func TestEveryRuleUndergoesTheCommonTest(t *testing.T) {
	for _, rule := range slopfix.AllRuleSpecs() {
		t.Run(rule.ID, func(t *testing.T) {
			require.NotEmpty(t, rule.Cases, "%s carries no case", rule.ID)
			for _, raw := range rule.Cases {
				// <input fixture, which includes the case(s) that the rule should detect>
				fixture, err := slopfix.Materialize(t.TempDir(), raw)
				require.NoError(t, err)

				// run ALL OTHER rules on the fixture, confirming that there are no detections other than the current rule we're testing
				assert.Empty(t, everyOtherRule(fixture, rule.ID), "%s: case %q: another rule detects the fixture", rule.ID, raw.Name)

				// run the rule on the fixture, confirming it is detected
				detected := rule.Detect(fixture)
				require.NotEmpty(t, detected, "%s: case %q: the rule does not detect its fixture", rule.ID, raw.Name)
				for _, f := range detected {
					assert.Equal(t, rule.ID, f.ID, "%s: case %q: the rule reports another rule's ID", rule.ID, raw.Name)
				}

				// run the rule on the result, confirming it was fixed (i.e. no longer detected)
				fixed := rule.Autofix(fixture)
				assert.Empty(t, rule.Detect(fixed), "%s: case %q: the rule still detects the result of its autofix", rule.ID, raw.Name)

				// run ALL OTHER rules on the "fixed" fixture, confirming there are STILL no detctions.
				assert.Empty(t, everyOtherRule(fixed, rule.ID), "%s: case %q: another rule detects the fixed fixture", rule.ID, raw.Name)
			}
		})
	}
}

// A rule with a detection needs an autofix. The registry refuses a nil autofix.
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

// The refusal above names the missing autofix, so it cannot pass on a probe
// malformed some other way.
func TestRegistrationNamesTheMissingAutofix(t *testing.T) {
	defer func() {
		assert.Contains(t, recover(), "no autofix")
	}()
	slopfix.RegisterRule(slopfix.RuleSpec{
		ID:       "probe/names-autofix",
		Category: slopfix.RuleSTE,
		Detect:   func(slopfix.RuleCase) []ste.Finding { return nil },
		Cases:    []slopfix.RuleCase{{Text: "x"}},
	})
}

// Every rule a category claims is registered, so --only never selects an empty
// set and reads as a clean file.
func TestEveryCategoryRuleIsRegistered(t *testing.T) {
	for _, category := range slopfix.AllRules() {
		for id := range slopfix.IDsFor(category).All() {
			_, ok := slopfix.RuleSpecByID(id)
			assert.True(t, ok, "%s is in category %s and is not registered", id, category)
		}
	}
}

// Every rule the registry lists carries an autofix. A rule added without one
// panics at registration, which a test above covers. This test asserts the
// in-force set, so a rule that somehow slips past the panic is caught here.
func TestEveryRegisteredRuleCarriesAnAutofix(t *testing.T) {
	var missing []string
	for _, rule := range slopfix.AllRuleSpecs() {
		if rule.Autofix == nil {
			missing = append(missing, rule.ID)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing, "these rules report a finding no autofix answers: %s", strings.Join(missing, ", "))
}

// The registry refuses a rule with no case, so no rule escapes the common test.
func TestRegistrationRefusesARuleWithNoCase(t *testing.T) {
	assert.Panics(t, func() {
		slopfix.RegisterRule(slopfix.RuleSpec{
			ID:       "probe/no-case",
			Category: slopfix.RuleSTE,
			Detect:   func(slopfix.RuleCase) []ste.Finding { return nil },
			Autofix:  func(c slopfix.RuleCase) slopfix.RuleCase { return c },
		})
	})
}

// Every rule ID a package can report is a registered rule. No detection
// reaches a reader without the autofix and the common test its rule carries.
func TestEveryReportableIDIsARegisteredRule(t *testing.T) {
	var missing []string
	for id := range slopfix.ReportableIDs().All() {
		if _, ok := slopfix.RuleSpecByID(id); !ok {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing, "these IDs are reported and are not registered rules: %s", strings.Join(missing, ", "))
}

// A finding under an ID no rule registered stops the run, at the point every
// report passes through.
func TestAFindingFromNoRegisteredRulePanics(t *testing.T) {
	assert.Panics(t, func() {
		slopfix.Registered([]ste.Finding{{ID: "probe/unregistered"}})
	})
}
