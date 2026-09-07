package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The step this rule protects, written both ways. They differ by a single key,
// which is the point: nothing in the gate's own output shows it.
const (
	neuteredStep = "on: push\n" +
		"jobs:\n" +
		"  lint:\n" +
		"    steps:\n" +
		"      - uses: actions/checkout@v4\n" +
		"      - uses: wow-look-at-my/slopfix@master\n" +
		"        continue-on-error: true\n"

	honestStep = "on: push\n" +
		"jobs:\n" +
		"  lint:\n" +
		"    steps:\n" +
		"      - uses: actions/checkout@v4\n" +
		"      - uses: wow-look-at-my/slopfix@master\n"
)

func TestAGateAllowedToFailIsReportedOnItsOwnLine(t *testing.T) {
	found := neuteredGates(neuteredStep)
	require.Len(t, found, 1)
	assert.Equal(t, IDNeuteredGate, found[0].ID)
	assert.Equal(t, 6, found[0].Line)
	assert.Contains(t, found[0].Detail, "slopfix")
	assert.Contains(t, found[0].Fix, "continue-on-error")
}

// The negative control. Without it the case above passes whether or not the
// rule can tell the steps apart.
func TestTheSameStepWithoutTheKeyIsNotReported(t *testing.T) {
	assert.Empty(t, neuteredGates(honestStep))
}

// continue-on-error on some other step says nothing about the gate.
func TestAnUnrelatedStepAllowedToFailIsNotReported(t *testing.T) {
	content := "on: push\n" +
		"jobs:\n" +
		"  lint:\n" +
		"    steps:\n" +
		"      - uses: some/flaky-upload@v1\n" +
		"        continue-on-error: true\n" +
		"      - uses: wow-look-at-my/slopfix@master\n"
	assert.Empty(t, neuteredGates(content))
}

func TestTheRuleRunsAsPartOfACheck(t *testing.T) {
	ids := make([]string, 0, 1)
	for _, finding := range Check(neuteredStep) {
		ids = append(ids, finding.ID)
	}
	assert.Contains(t, ids, IDNeuteredGate)
}
