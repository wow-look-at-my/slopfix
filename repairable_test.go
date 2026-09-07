package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// A rule with a repair and a rule without it come back labelled apart. That
// split is what lets a reporting caller and a repairing caller share the wire
// contract without either carrying a list.
func TestARuleWithARepairIsLabelledApartFromOneWithout(t *testing.T) {
	assert.True(t, slopfix.Repairable(ste.IDSemicolon))
	assert.True(t, slopfix.Repairable(slopfix.IDHardWrap))

	assert.False(t, slopfix.Repairable(ste.IDSentenceCap))
	assert.False(t, slopfix.Repairable(workflow.IDCommentBlock))
	assert.False(t, slopfix.Repairable(workflow.IDNeuteredGate))
}

// The defect this property exists for. A stale count is reported under the
// prose rule's ID and repaired under the counts rule's, so a caller reading the
// finding's own ID alone reports what another hook already repaired.
func TestAStaleCountIsRepairableUnderEitherID(t *testing.T) {
	assert.True(t, slopfix.Repairable(ste.IDStaleCount))
	assert.True(t, slopfix.Repairable(slopfix.IDInventoryCount))
}

// The negative control. An unknown name is not repairable, so the case above
// passes on the answer rather than on a set that says yes to everything.
func TestAnUnknownRuleIsNotRepairable(t *testing.T) {
	assert.False(t, slopfix.Repairable("ste/not-a-real-rule"))
}

// Every rule the report can carry has an answer, and the two sides are both
// occupied. A property that is true everywhere splits nothing.
func TestBothSidesOfTheSplitAreOccupied(t *testing.T) {
	var repairs, reports int
	for id := range slopfix.AllIDs().All() {
		if slopfix.Repairable(id) {
			repairs++
			continue
		}
		reports++
	}
	require.Positive(t, repairs)
	require.Positive(t, reports)
}
