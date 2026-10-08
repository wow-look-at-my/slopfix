package slopfix

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
		Cases: []RuleCase{workflowCase("neutered-gate", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+workflowGate+
			"    steps:\n      - uses: wow-look-at-my/slopfix@master\n        continue-on-error: true\n")},
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
