// rule_yaml_neuteredgate.go holds the rule yaml neuteredgate code.

package slopfix

import _ "embed"

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// gateAllowedToFail is the case's step that runs the gate under the key that
// lets it fail.
var gateAllowedToFail = strings.Join([]string{
	"    steps:",
	"      - uses: wow-look-at-my/slopfix@master",
	"        continue-on-" + "error: true",
}, "\n") + "\n"

// yaml/neutered-gate: a step that runs the gate with continue-on-error, so a
// finding cannot fail the run. The repair drops the key.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDNeuteredGate,
		Category: RuleWorkflow,
		Detect:   detectNeuteredGate,
		Autofix:  autofixNeuteredGate,
		Cases: []RuleCase{workflowCase("neutered-gate", workflowHeader()+embeddedYml11+
			gateAllowedToFail)},
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
var embeddedYml11 string
