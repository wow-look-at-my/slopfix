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
				"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n"+
				embeddedYml5),
			workflowCase("run-script-syntax-unclosed", workflowHeader()+
				"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n"+
				embeddedYml6),
			workflowCase("run-script-syntax-run-row", workflowHeader()+
				"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo (\n"),
			workflowCase("run-script-syntax-stray-closer", workflowHeader()+
				"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n"+
				embeddedYml7),
			workflowCase("run-script-syntax-sh-array", workflowHeader()+
				"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n"+
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

//go:embed testdata/embedded-5.yml
var embeddedYml5 string

//go:embed testdata/embedded-6.yml
var embeddedYml6 string

//go:embed testdata/embedded-7.yml
var embeddedYml7 string
