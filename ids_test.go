package slopfix_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// The identity is a set, so a name declared again collapses rather than sitting
// in the list as a duplicate. A slice cannot notice the duplicate at all, and every
// consumer asked whether a name was in it regardless.
func TestARuleIDCannotBeDeclaredTwice(t *testing.T) {
	every := slopfix.AllIDs()
	assert.Equal(t, len(slices.Sorted(every.All())), every.Len())

	// Union folds a duplicate away. A slice built the same way keeps both.
	assert.True(t, every.Union(workflow.AllIDs).Equal(every))
	assert.True(t, every.Union(ste.AllIDs).Equal(every))
}

// The negative control. A set the caller really adds a NEW name to grows, so
// the case above passes because the duplicate collapsed rather than because
// nothing was added.
func TestANewRuleIDDoesGrowTheSet(t *testing.T) {
	every := slopfix.AllIDs()
	grown := every.Clone()
	grown.Add("ste/not-a-real-rule")
	assert.Equal(t, every.Len()+1, grown.Len())
}

// A set has no order of its own, so the list a person reads has to be built.
// An unstable listing puts a different rule at the head on every run.
func TestTheListingAPersonReadsIsAlphabeticalAndStable(t *testing.T) {
	first := slopfix.Listed(slopfix.AllIDs())
	require.NotEmpty(t, first)
	for range 5 {
		assert.Equal(t, first, slopfix.Listed(slopfix.AllIDs()))
	}

	names := strings.Split(first, ", ")
	assert.True(t, slices.IsSorted(names), "the listing is not alphabetical: %q", first)
	assert.Contains(t, names, workflow.IDCommentBlock)
	assert.Contains(t, names, ste.IDSemicolon)
	assert.Contains(t, names, slopfix.IDHardWrap)
}

// The negative control for the case above. An unsorted listing of the same
// names really does differ, so the assertion can fail.
func TestAnUnsortedListingIsNotWhatIsPrinted(t *testing.T) {
	names := slices.Sorted(slopfix.AllIDs().All())
	require.Greater(t, len(names), 1)
	slices.Reverse(names)
	assert.NotEqual(t, strings.Join(names, ", "), slopfix.Listed(slopfix.AllIDs()))
}

// Every category answers with a set too, so a caller never falls back to a scan
// over a materialised slice.
func TestEveryCategoryAnswersWithASet(t *testing.T) {
	assert.True(t, slopfix.IDsFor(slopfix.RuleSTE).Contains(ste.IDSemicolon))
	assert.True(t, slopfix.IDsFor(slopfix.RuleWrap).Contains(slopfix.IDHardWrap))
	assert.True(t, slopfix.IDsFor(slopfix.RuleTombstones).Contains(tombstones.IDVolume))
	assert.False(t, slopfix.IDsFor(slopfix.RuleCounts).Contains(ste.IDSemicolon))
	assert.True(t, slopfix.IDsFor(slopfix.Rule("nosuch")).IsEmpty())
}
