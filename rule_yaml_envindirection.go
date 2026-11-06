// yaml/env-indirection: a step that sets an env var only to carry an expression into its run script.

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
			workflowCase("env-indirection", workflowHeader()+embeddedYml11+
				embeddedYml12),
			// From buildhost's buildhost-publish action: the entry only carries an input into the script.
			{Name: "yaml/env-indirection-typescript", Path: ".github/actions/buildhost-publish/action.yml", Text: scriptActionManifest(
				embeddedYml13,
				"          const inputAptDepends: string = process.env.BUILDHOST_APT_DEPENDS ?? \"\";\n"+
					"          const project: string = process.env.BUILDHOST_PROJECT || \"repo\";\n"+
					"          if (!process.env.BUILDHOST_PROJECT) {\n            core.info(\"no project input\");\n          }\n"+
					"          if (inputAptDepends && process.env.BUILDHOST_PROJECT) {\n            core.info(\"project with prerequisites\");\n          }\n"+
					"          core.info(`Publishing ${process.env.BUILDHOST_PROJECT} as ${project} with ${inputAptDepends}`);\n")},
			// A `??` arm that goes on past its line stays.
			workflowCase("env-indirection-github-script", workflowHeader()+embeddedYml14+
				embeddedYml15+
				embeddedYml16+
				"            core.info(process.env.SHA ??\n              \"none\");\n"),
			envKeptCase(),
		},
	})
}

// scriptActionManifest is a composite action whose one step runs a TypeScript
// script with the given env block and script lines.
func scriptActionManifest(env, script string) string {
	return scriptActionHead() + env + "      with:\n        script: |\n" + script
}

// scriptActionHead is the composite action up to its TypeScript step's uses line.
func scriptActionHead() string {
	return scriptActionHeadYml
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

//go:embed testdata/script-action-head.yml
var scriptActionHeadYml string

//go:embed testdata/embedded-11.yml
var embeddedYml11 string

//go:embed testdata/embedded-12.yml
var embeddedYml12 string

//go:embed testdata/embedded-13.yml
var embeddedYml13 string

//go:embed testdata/embedded-14.yml
var embeddedYml14 string

//go:embed testdata/embedded-15.yml
var embeddedYml15 string

//go:embed testdata/embedded-16.yml
var embeddedYml16 string

//go:embed testdata/env-kept-case.yml
var envKeptCaseYml string
