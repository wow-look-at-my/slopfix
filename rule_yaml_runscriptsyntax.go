package slopfix

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/run-script-syntax: a run script the shell it runs in cannot parse. A
// step on the default branch alone fails only after a merge. The repair gives
// an empty body the no-op `:`, closes what the script left open, and comments
// out any other line the shell rejects.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDRunScriptSyntax,
		Category: RuleWorkflow,
		Detect:   detectRunScriptSyntax,
		Autofix:  autofixRunScriptSyntax,
		Cases: []RuleCase{
			workflowCase("run-script-syntax-empty-body", workflowHeader()+
				"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n"+
				"      - run: |\n          if [ -z \"$WHEEL\" ]; then\n          fi\n"),
			workflowCase("run-script-syntax-unclosed", workflowHeader()+
				"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n"+
				"      - run: |\n          if true; then\n            echo hi\n"),
			workflowCase("run-script-syntax-run-row", workflowHeader()+
				"jobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo (\n"),
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
