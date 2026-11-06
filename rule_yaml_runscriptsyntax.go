// rule_yaml_runscriptsyntax.go holds the rule yaml runscriptsyntax code.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/run-script-syntax: a run script the shell it runs in cannot parse. A
// step on the default branch alone fails only after a merge. The rule reports
// the errors its repair mends. An empty body gets the no-op `:`, and an
// unclosed construct gets its closer. A stray closing word is dropped, and a
// stray operator is quoted. An `sh` step that needs bash names it.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDRunScriptSyntax,
		Category: RuleWorkflow,
		Detect:   detectRunScriptSyntax,
		Autofix:  autofixRunScriptSyntax,
		Cases: []RuleCase{
			workflowCase("run-script-syntax-empty-body", workflowHeader()+
				embeddedYml34+
				embeddedYml35),
			workflowCase("run-script-syntax-unclosed", workflowHeader()+
				embeddedYml36+
				embeddedYml37),
			workflowCase("run-script-syntax-run-row", workflowHeader()+
				embeddedYml38),
			workflowCase("run-script-syntax-stray-closer", workflowHeader()+
				embeddedYml39+
				embeddedYml40),
			workflowCase("run-script-syntax-sh-array", workflowHeader()+
				embeddedYml41+
				"      - shell: sh\n        run: |\n          FILES=(a b)\n          echo \"${FILES[0]}\"\n"),
		},
	})
}

// detectRunScriptSyntax answers every run script that does not parse as shell.
func detectRunScriptSyntax(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDRunScriptSyntax)
}

// autofixRunScriptSyntax writes each such script as one that parses.
func autofixRunScriptSyntax(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDRunScriptSyntax)
}

//go:embed testdata/embedded-34.yml
var embeddedYml34 string

//go:embed testdata/embedded-35.yml
var embeddedYml35 string

//go:embed testdata/embedded-36.yml
var embeddedYml36 string

//go:embed testdata/embedded-37.yml
var embeddedYml37 string

//go:embed testdata/embedded-38.yml
var embeddedYml38 string

//go:embed testdata/embedded-39.yml
var embeddedYml39 string

//go:embed testdata/embedded-40.yml
var embeddedYml40 string

//go:embed testdata/embedded-41.yml
var embeddedYml41 string
