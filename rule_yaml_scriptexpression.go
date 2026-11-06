// yaml/script-expression: a TypeScript step script that writes a context value in as a ${{ }} expression.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/script-expression: a TypeScript step script that writes a context value
// in as a ${{ }} expression. The action hands the script each context as an
// object, so the repair reads the object and passes the context under with.
func init() {
	steps := func(with, script string) string {
		return workflowHeader() + embeddedYml52 +
			embeddedYml53 +
			"      - uses: wow-look-at-my/actions@typescript#latest\n        with:\n" + with + "          script: |\n" + script
	}
	RegisterRule(RuleSpec{
		ID:       workflow.IDScriptExpression,
		Category: RuleWorkflow,
		Detect:   detectScriptExpression,
		Autofix:  autofixScriptExpression,
		Cases: []RuleCase{
			// From go-toolchain's action: the script parsed an input it had serialized.
			{
				Name: "yaml/script-expression-input", Path: ".github/actions/buildhost-publish/action.yml",
				Text: scriptActionWith("", "          const raw: string = String(${{ toJSON(inputs.project) }});\n          core.info(raw);\n"),
				Want: scriptActionWith("        inputs: ${{ toJSON(inputs) }}\n", "          const raw: string = String(inputs.project);\n          core.info(raw);\n"),
			},
			{
				Name: "yaml/script-expression-step-output", Path: ".github/workflows/ci.yml",
				Text: steps("", "            const file: string = String(${{ toJSON(format('{0}', steps.download.outputs.path)) }});\n"+
					"            core.info(\"${{ matrix.os }}\" + '${{ github.sha }}' + file);\n"),
				Want: steps("          matrix: ${{ toJSON(matrix) }}\n          steps: ${{ toJSON(steps) }}\n",
					"            const file: string = String(steps.download.outputs.path ?? \"\");\n"+
						"            core.info(String(matrix.os ?? \"\") + String(github.sha ?? \"\") + file);\n"),
			},
			{
				Name: "yaml/script-expression-passed", Path: ".github/workflows/ci.yml",
				Text: steps("          steps: ${{ toJSON(steps) }}\n", "            core.info(${{ toJSON(steps.download.outputs.path) }});\n"),
				Want: steps("          steps: ${{ toJSON(steps) }}\n", "            core.info(steps.download.outputs.path);\n"),
			},
			scriptExpressionKept(),
		},
	})
}

// scriptActionWith is scriptActionManifest with entries under with ahead of the script.
func scriptActionWith(with, script string) string {
	return scriptActionHead() + "      with:\n" + with + "        script: |\n" + script
}

// scriptExpressionKept holds expressions the rule leaves. A github-script step
// gets no context object. A with entry that names the context another way
// blocks it, and text around an expression is not one value.
func scriptExpressionKept() RuleCase {
	c := workflowCase("script-expression-kept", workflowHeader()+scriptExpressionKeptYml+
		"      - uses: actions/github-script@v7\n        with:\n          script: |\n            core.info(${{ toJSON(github.sha) }});\n"+
		"      - uses: wow-look-at-my/actions@typescript#latest\n        with:\n          steps: '{}'\n          script: |\n"+
		"            core.info(${{ toJSON(steps.a.outputs.b) }});\n            core.info(\"sha ${{ github.sha }}\");\n"+
		"            core.info(${{ toJSON(github.workspace) }});\n")
	c.Unchanged = true
	return c
}

// detectScriptExpression answers every expression a TypeScript script reads where the context object holds the value.
func detectScriptExpression(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDScriptExpression)
}

// autofixScriptExpression reads the context object in place of each expression.
func autofixScriptExpression(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDScriptExpression)
}

//go:embed testdata/embedded-44.yml
var embeddedYml52 string

//go:embed testdata/embedded-45.yml
var embeddedYml53 string

//go:embed testdata/script-expression-kept.yml
var scriptExpressionKeptYml string
