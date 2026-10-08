package slopfix_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// EVERY rule carries a repair. A rule that only reports hands the reader a
// finding and no way out of it, and the tool exists to write the repair.
//
// This is the check the gate runs. A rule added without a repair fails the
// build that adds it, while its author still holds the context to write it.
func TestEveryRuleCarriesARepair(t *testing.T) {
	var missing []string
	for id := range slopfix.EveryID().All() {
		if !slopfix.Repairable(id) {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	assert.Empty(t, missing, "these rules report a finding no repair answers: %s", strings.Join(missing, ", "))
}

// A repair is claimed rule by rule, so an unknown name is answered rather than
// guessed.
func TestARepairIsClaimedRuleByRule(t *testing.T) {
	assert.True(t, slopfix.Repairable(ste.IDSemicolon))
	assert.True(t, slopfix.Repairable(slopfix.IDHardWrap))
	assert.True(t, slopfix.Repairable(workflow.IDCommentBlock))
	assert.True(t, slopfix.Repairable(workflow.IDNeuteredGate))
	assert.True(t, slopfix.Repairable(ste.IDPassive))
	assert.True(t, slopfix.Repairable(tombstones.IDVolume))
	assert.True(t, slopfix.Repairable(slopfix.IDNearDuplicate))
}

// The defect this property exists for. A stale count is reported under the
// prose rule's ID and repaired under the counts rule's, and both answer yes.
func TestAStaleCountIsRepairableUnderEitherID(t *testing.T) {
	assert.True(t, slopfix.Repairable(ste.IDStaleCount))
	assert.True(t, slopfix.Repairable(slopfix.IDInventoryCount))
}

// The negative control. An unknown name is not repairable, so the cases above
// pass on the answer rather than on a set saying yes.
func TestAnUnknownRuleIsNotRepairable(t *testing.T) {
	assert.False(t, slopfix.Repairable("ste/not-a-real-rule"))
}

// The rule set is not empty, so the coverage case above passes on rules rather
// than on an empty walk.
func TestTheRuleSetIsNotEmpty(t *testing.T) {
	require.NotEmpty(t, slopfix.AllIDs().Len())
	require.NotEmpty(t, slopfix.EveryID().Len())
}
