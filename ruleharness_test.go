package slopfix_test

import (
	"os"
	"path/filepath"
	"sort"
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

// checkEveryOtherRule reports whether any rule other than id fires on c.
func checkEveryOtherRule(t *testing.T, c slopfix.RuleCase, id string) []ste.Finding {
	t.Helper()
	var foreign []ste.Finding
	for _, rule := range slopfix.AllRuleSpecs() {
		if rule.ID == id {
			continue
		}
		for _, f := range rule.Detect(c) {
			foreign = append(foreign, f)
		}
	}
	return foreign
}

// TestEveryRuleIsDetectedRepairedAndAlone is the common harness. Steps, no
// deviation:

//  1. Run ALL OTHER rules on the fixture, confirming none reports a finding
//     other than the rule under test.
//  2. Run the rule on the fixture, confirming it detects its own case.
//  3. Run the rule's autofix on the fixture.
//  4. Run the rule on the result, confirming it no longer detects anything.
//  5. Run ALL OTHER rules on the fixed fixture, confirming no foreign rule
//     reports a finding on the result.
func TestEveryRuleIsDetectedRepairedAndAlone(t *testing.T) {
	for _, rule := range slopfix.AllRuleSpecs() {
		rule := rule
		t.Run(rule.ID, func(t *testing.T) {
			require.NotEmpty(t, rule.Cases, "%s carries no case", rule.ID)
			for _, raw := range rule.Cases {
				c := materialize(t, raw)
				// A case the rule must leave as written fires nothing, and
				// TestEveryUnchangedCaseRoundTrips holds its own property.
				if c.Unchanged {
					continue
				}

				foreign := checkEveryOtherRule(t, c, rule.ID)
				assert.Empty(t, foreign, "%s: case %q trips another rule before the repair", rule.ID, c.Name)

				before := rule.Detect(c)
				require.NotEmpty(t, before, "%s: case %q does not fire its detection", rule.ID, c.Name)
				for _, f := range before {
					assert.Equal(t, rule.ID, f.ID, "%s: case %q reported another rule", rule.ID, c.Name)
				}

				fixed := rule.Autofix(c)

				after := rule.Detect(fixed)
				assert.Empty(t, after, "%s: case %q still detects after its own autofix", rule.ID, c.Name)

				foreignAfter := checkEveryOtherRule(t, fixed, rule.ID)
				assert.Empty(t, foreignAfter, "%s: case %q trips another rule after the repair", rule.ID, c.Name)
			}
		})
	}
}

// TestEveryUnchangedCaseRoundTrips is the harness for the cases a rule must
// leave alone: the text carries no finding. The autofix writes it back byte
// for byte. A repair that rewords prose the author wrote fails here.
func TestEveryUnchangedCaseRoundTrips(t *testing.T) {
	seen := 0
	for _, rule := range slopfix.AllRuleSpecs() {
		rule := rule
		for _, raw := range rule.Cases {
			if !raw.Unchanged {
				continue
			}
			seen++
			c := materialize(t, raw)
			t.Run(rule.ID+"/"+c.Name, func(t *testing.T) {
				require.NotEmpty(t, c.Text, "%s: an unchanged case carries no text", c.Name)
				assert.Empty(t, rule.Detect(c), "%s: case %q must fire no detection", rule.ID, c.Name)
				assert.Equal(t, c.Text, rule.Autofix(c).Text, "%s: case %q must round-trip unchanged", rule.ID, c.Name)
			})
		}
	}
	require.NotZero(t, seen, "no rule carries an unchanged case")
}

// A rule whose finding tells the reader that `slopfix fix` repairs it owes a
// working autofix. A message that advertises a repair the rule does not carry
// fails here rather than shipping a promise the tool cannot keep.
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
			for _, raw := range rule.Cases {
				c := materialize(t, raw)
				after := rule.Autofix(c)
				assert.Empty(t, rule.Detect(after), "%s: case %q still detects after the promised autofix", rule.ID, c.Name)
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

// The harness reaches every registered rule. A rule added with no driven case
// fails here rather than shipping a detection the harness never proves.
func TestTheHarnessCoversEveryRegisteredRule(t *testing.T) {
	covered := set.New[string]()
	for _, rule := range slopfix.AllRuleSpecs() {
		for _, c := range rule.Cases {
			if !c.Unchanged {
				covered.Add(rule.ID)
				break
			}
		}
	}
	for _, rule := range slopfix.AllRuleSpecs() {
		assert.True(t, covered.Contains(rule.ID), "%s carries no case the harness drives", rule.ID)
	}
}
