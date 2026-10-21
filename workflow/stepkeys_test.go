package workflow

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GitHub rejects the whole file for a repeated step key, before it makes any
// job, so the run that would have reported it never starts.
func TestAStepThatNamesAKeyTwiceIsReported(test *testing.T) {
	findings := duplicateStepKeys(testAStepThatNamesAKeyTwiceIsReportedYml)

	require.Len(test, findings, 1)
	assert.Equal(test, IDDuplicateStepKey, findings[0].ID)
	assert.Equal(test, 7, findings[0].Line)
	assert.Equal(test, "name", findings[0].Detail)
}

// A key one level in belongs to with:, env: or another mapping, where the same
// name is free to appear again.
func TestAKeyNestedUnderAStepMayRepeat(test *testing.T) {
	content := testAKeyNestedUnderAStepMayRepeatYml

	assert.Empty(test, duplicateStepKeys(content))
}

// Each step starts its own key set, so steps naming the same key is ordinary.
func TestTwoStepsMayNameTheSameKey(test *testing.T) {
	content := testTwoStepsMayNameTheSameKeyYml

	assert.Empty(test, duplicateStepKeys(content))
}

// The key on the dash row is the step's own, so a repeat of it counts.
func TestTheKeyOnTheDashRowCounts(test *testing.T) {
	findings := duplicateStepKeys(testTheKeyOnTheDashRowCountsYml)

	require.Len(test, findings, 1)
	assert.Equal(test, 6, findings[0].Line)
	assert.Equal(test, "run", findings[0].Detail)
}

//go:embed testdata/test-a-step-that-names-a-key-twice-is-reported.yml
var testAStepThatNamesAKeyTwiceIsReportedYml string

//go:embed testdata/test-a-key-nested-under-a-step-may-repeat.yml
var testAKeyNestedUnderAStepMayRepeatYml string

//go:embed testdata/test-two-steps-may-name-the-same-key.yml
var testTwoStepsMayNameTheSameKeyYml string

//go:embed testdata/test-the-key-on-the-dash-row-counts.yml
var testTheKeyOnTheDashRowCountsYml string
