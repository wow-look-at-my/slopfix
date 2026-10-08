package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// GitHub rejects the whole file for a repeated step key, before it makes any
// job, so the run that would have reported it never starts.
func TestAStepThatNamesAKeyTwiceIsReported(test *testing.T) {
	findings := workflow.Check("on: push\njobs:\n  build:\n    steps:\n      - name: one\n        run: echo hi\n        name: two\n")

	require.Len(test, findings, 1)
	assert.Equal(test, workflow.IDDuplicateStepKey, findings[0].ID)
	assert.Equal(test, 7, findings[0].Line)
	assert.Equal(test, "name", findings[0].Detail)
}

// A key one level in belongs to with:, env: or another mapping, where the same
// name is free to appear again.
func TestAKeyNestedUnderAStepMayRepeat(test *testing.T) {
	content := "on: push\njobs:\n  build:\n    steps:\n      - uses: some/action\n        with:\n          name: one\n        env:\n          name: two\n"

	assert.Empty(test, workflow.Check(content))
}

// Each step starts its own key set, so steps naming the same key is
// ordinary.
func TestTwoStepsMayNameTheSameKey(test *testing.T) {
	content := "on: push\njobs:\n  build:\n    steps:\n      - name: one\n        run: echo hi\n      - name: two\n        run: echo bye\n"

	assert.Empty(test, workflow.Check(content))
}

// The key on the dash row is the step's own, so a repeat of it counts.
func TestTheKeyOnTheDashRowCounts(test *testing.T) {
	findings := workflow.Check("on: push\njobs:\n  build:\n    steps:\n      - run: echo hi\n        run: echo bye\n")

	require.Len(test, findings, 1)
	assert.Equal(test, 6, findings[0].Line)
	assert.Equal(test, "run", findings[0].Detail)
}
