// yaml/duplicate-step-key: a step that names one key twice.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/duplicate-step-key: a step that names one key twice. GitHub rejects the
// whole file for it, before any job starts. The repair keeps the last pair.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDDuplicateStepKey,
		Category: RuleWorkflow,
		Detect:   detectDuplicateStepKey,
		Autofix:  autofixDuplicateStepKey,
		Cases: []RuleCase{
			workflowCase("duplicate-step-key-first-key", workflowHeader()+
				embeddedYml9+
				"      - run: echo one\n        name: build\n        run: echo two\n"),
			workflowCase("duplicate-step-key-block-value", workflowHeader()+
				embeddedYml10+
				"      - name: build\n        run: |\n          echo a\n          echo b\n        run: echo c\n"),
		},
	})
}

// detectDuplicateStepKey answers every step key written twice at the step's own level.
func detectDuplicateStepKey(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDDuplicateStepKey)
}

// autofixDuplicateStepKey deletes every pair but the last of a repeated key.
func autofixDuplicateStepKey(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDDuplicateStepKey)
}

//go:embed testdata/embedded-9.yml
var embeddedYml9 string

//go:embed testdata/embedded-10.yml
var embeddedYml10 string
