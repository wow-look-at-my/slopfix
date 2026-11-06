package workflow

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The step this rule protects, written both ways.
const ()

func TestAGateAllowedToFailIsReportedOnItsOwnLine(t *testing.T) {
	found := neuteredGates(neuteredStep)
	require.Len(t, found, 1)
	assert.Equal(t, IDNeuteredGate, found[0].ID)
	assert.Equal(t, 6, found[0].Line)
	assert.Contains(t, found[0].Detail, "slopfix")
	assert.Contains(t, found[0].Fix, "continue-on-error")
}

// The negative control. Without it the case above passes whether the rule can
// tell the steps apart.
func TestTheSameStepWithoutTheKeyIsNotReported(t *testing.T) {
	assert.Empty(t, neuteredGates(honestStep))
}

// continue-on-error on some other step says nothing about the gate.
func TestAnUnrelatedStepAllowedToFailIsNotReported(t *testing.T) {
	content := testAnUnrelatedStepAllowedToFailIsNotReportedYml
	assert.Empty(t, neuteredGates(content))
}

func TestTheRuleRunsAsPartOfACheck(t *testing.T) {
	ids := make([]string, 0, 1)
	for _, finding := range Check(neuteredStep) {
		ids = append(ids, finding.ID)
	}
	assert.Contains(t, ids, IDNeuteredGate)
}

//go:embed testdata/neutered-step.yml
var neuteredStep string

//go:embed testdata/honest-step.yml
var honestStep string

//go:embed testdata/test-an-unrelated-step-allowed-to-fail-is-not-reported.yml
var testAnUnrelatedStepAllowedToFailIsNotReportedYml string
