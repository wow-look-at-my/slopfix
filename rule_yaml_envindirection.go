// rule_yaml_envindirection.go holds the rule yaml envindirection code.

package slopfix

import _ "embed"

import (
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// yaml/env-indirection: a step that sets an env var only to carry an expression
// into its run script. This is or into the script of a TypeScript or
// github-script step. This hides the value from the reader. The repair inlines
// it.
func init() {
	RegisterRule(RuleSpec{
		ID:       workflow.IDEnvIndirection,
		Category: RuleWorkflow,
		Detect:   detectEnvIndirection,
		Autofix:  autofixEnvIndirection,
		Cases: []RuleCase{
			workflowCase("env-indirection", workflowHeader()+embeddedYml5+
				embeddedYml6),
			// From buildhost's buildhost-publish action: the entry only carries an input into the script.
			{Name: "yaml/env-indirection-typescript", Path: ".github/actions/buildhost-publish/action.yml", Text: scriptActionManifest(
				embeddedYml7,
				"          const inputAptDepends: string = process.env.BUILDHOST_APT_DEPENDS ?? \"\";\n"+
					"          const project: string = process.env.BUILDHOST_PROJECT || \"repo\";\n"+
					"          if (!process.env.BUILDHOST_PROJECT) {\n            core.info(\"no project input\");\n          }\n"+
					"          if (inputAptDepends && process.env.BUILDHOST_PROJECT) {\n            core.info(\"project with prerequisites\");\n          }\n"+
					"          core.info(`Publishing ${process.env.BUILDHOST_PROJECT} as ${project} with ${inputAptDepends}`);\n")},
			// A `??` arm that goes on past its line stays.
			workflowCase("env-indirection-github-script", workflowHeader()+embeddedYml8+
				embeddedYml9+
				embeddedYml10+
				"            core.info(process.env.SHA ??\n              \"none\");\n"),
			envKeptCase(),
		},
	})
}

// scriptActionManifest is a composite action whose one step runs a TypeScript
// script with the given env block and script lines.
func scriptActionManifest(env, script string) string {
	return scriptActionManifestYml +
		env + "      with:\n        script: |\n" + script
}

// envKeptCase holds entries with a job to do. A script that assigns the
// variable needs it in the environment. A name another step reads keeps its
// entry too.
func envKeptCase() RuleCase {
	c := RuleCase{Name: "yaml/env-indirection-typescript-kept", Path: ".github/actions/buildhost-publish/action.yml", Text: scriptActionManifest(
		envKeptCaseYml,
		"          process.env.BUILDHOST_APT_DEPENDS ??= \"\";\n"+
			"          core.info(`Publishing ${process.env.BUILDHOST_PROJECT} with ${process.env.BUILDHOST_APT_DEPENDS}`);\n"+
			"    - shell: bash\n      run: echo \"$BUILDHOST_PROJECT\"\n")}
	c.Unchanged = true
	return c
}

// detectEnvIndirection answers every step that sets an env var it reads once.
func detectEnvIndirection(c RuleCase) []ste.Finding {
	return caseFindings(c, workflow.IDEnvIndirection)
}

// autofixEnvIndirection inlines the env var's value into its one read.
func autofixEnvIndirection(c RuleCase) RuleCase {
	return caseAutofix(c, workflow.IDEnvIndirection)
}

//go:embed testdata/embedded-5.yml
var embeddedYml5 string

//go:embed testdata/embedded-6.yml
var embeddedYml6 string

//go:embed testdata/embedded-7.yml
var embeddedYml7 string

//go:embed testdata/embedded-8.yml
var embeddedYml8 string

//go:embed testdata/embedded-9.yml
var embeddedYml9 string

//go:embed testdata/embedded-10.yml
var embeddedYml10 string

//go:embed testdata/script-action-manifest.yml
var scriptActionManifestYml string

//go:embed testdata/env-kept-case.yml
var envKeptCaseYml string
