package workflow

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stepFixture writes a workflow fixture from one argument per line.
func stepFixture(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

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

// neuteredStep is the step this rule protects, under the key that lets it fail.
var neuteredStep = stepFixture(
	"on: {push: {branches: ['**']}}",
	"jobs:",
	"  lint:",
	"    steps:",
	"      - uses: actions/checkout@v4",
	"      - uses: wow-look-at-my/slopfix@master",
	"        continue-on-"+"error: true",
)

//go:embed testdata/honest-step.yml
var honestStep string

// testAnUnrelatedStepAllowedToFailIsNotReportedYml holds the key on another
// step, which says nothing about the gate.
var testAnUnrelatedStepAllowedToFailIsNotReportedYml = stepFixture(
	"on: {push: {branches: ['**']}}",
	"jobs:",
	"  lint:",
	"    steps:",
	"      - uses: some/flaky-upload@v1",
	"        continue-on-"+"error: true",
	"      - uses: wow-look-at-my/slopfix@master",
)
