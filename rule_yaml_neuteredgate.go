// yaml/neutered-gate: a step that runs the gate with continue-on-error, so a finding cannot fail the run.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/neutered-gate: a step that runs the gate with continue-on-error, so a
// finding cannot fail the run. The repair drops the key.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDNeuteredGate,
		Category: RuleWorkflow,
		Detect:   detectNeuteredGate,
		Autofix:  autofixNeuteredGate,
		Cases: []RuleCase{workflowCase("neutered-gate", workflowHeader()+embeddedYml24+
			embeddedYml25)},
	})
}

// detectNeuteredGate answers every gate step carrying continue-on-error.
func detectNeuteredGate(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDNeuteredGate)
}

// autofixNeuteredGate drops the continue-on-error key.
func autofixNeuteredGate(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDNeuteredGate)
}

//go:embed testdata/embedded-11.yml
var embeddedYml24 string

//go:embed testdata/embedded-23.yml
var embeddedYml25 string
