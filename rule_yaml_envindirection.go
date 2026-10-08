package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/env-indirection: a step that sets an env var and reads it once in the
// same script, which hides the value from the reader. The repair inlines it.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDEnvIndirection,
		Category: RuleWorkflow,
		Detect:   detectEnvIndirection,
		Autofix:  autofixEnvIndirection,
		Cases: []RuleCase{workflowCase("env-indirection", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+workflowGate+
			"    steps:\n      - env:\n          OUT: ${{ github.sha }}\n        run: echo \"$OUT\"\n")},
	})
}

// detectEnvIndirection answers every step that sets an env var it reads once.
func detectEnvIndirection(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDEnvIndirection)
}

// autofixEnvIndirection inlines the env var's value into its one read.
func autofixEnvIndirection(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDEnvIndirection)
}
