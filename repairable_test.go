package slopfix_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// EVERY rule carries a repair. A rule that only reports hands the reader a
// finding and no way out of it, and the tool exists to write the repair.
//
// This is the check the gate runs: a rule added without one fails the build
// that adds it, which is the moment its author is still holding the context to
// write it.
func TestEveryRuleCarriesARepair(t *testing.T) {
	var missing []string
	for id := range slopfix.AllIDs().All() {
		if !slopfix.Repairable(id) {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing, "these rules report a finding no repair answers: %s", strings.Join(missing, ", "))
}

// The negative control. An unknown name is not repairable, so the case above
// passes on the rules that exist rather than on a set saying yes to anything.
func TestARepairIsClaimedRuleByRule(t *testing.T) {
	assert.True(t, slopfix.Repairable(ste.IDSemicolon))
	assert.True(t, slopfix.Repairable(slopfix.IDHardWrap))
	assert.True(t, slopfix.Repairable(workflow.IDCommentBlock))
	assert.True(t, slopfix.Repairable(workflow.IDNeuteredGate))
}

// The defect this property exists for. A stale count is reported under the
// prose rule's ID and repaired under the counts rule's.
func TestAStaleCountIsRepairableUnderEitherID(t *testing.T) {
	assert.True(t, slopfix.Repairable(ste.IDStaleCount))
	assert.True(t, slopfix.Repairable(slopfix.IDInventoryCount))
}

// The negative control. An unknown name is not repairable, so the case above
// passes on the answer rather than on a set saying yes.
func TestAnUnknownRuleIsNotRepairable(t *testing.T) {
	assert.False(t, slopfix.Repairable("ste/not-a-real-rule"))
}

// The rule set is not empty, so the coverage case above passes on rules rather
// than on an empty walk.
func TestTheRuleSetIsNotEmpty(t *testing.T) {
	require.NotEmpty(t, slopfix.AllIDs().Len())
}
