package slopfix

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
			workflowCase("env-indirection", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+
				"    steps:\n      - env:\n          OUT: ${{ github.sha }}\n        run: echo \"$OUT\"\n"),
			// From buildhost's buildhost-publish action: the entry only carries an input into the script.
			{Name: "yaml/env-indirection-typescript", Path: ".github/actions/buildhost-publish/action.yml", Text: scriptActionManifest(
				"      env:\n        # An env var carries the value, so no character in it can break the script.\n"+
					"        BUILDHOST_PROJECT: ${{ inputs.project }}\n        BUILDHOST_APT_DEPENDS: ${{ inputs.apt_depends }}\n",
				"          const inputAptDepends: string = process.env.BUILDHOST_APT_DEPENDS ?? \"\";\n"+
					"          const project: string = process.env.BUILDHOST_PROJECT || \"repo\";\n"+
					"          if (!process.env.BUILDHOST_PROJECT) {\n            core.info(\"no project input\");\n          }\n"+
					"          if (inputAptDepends && process.env.BUILDHOST_PROJECT) {\n            core.info(\"project with prerequisites\");\n          }\n"+
					"          core.info(`Publishing ${process.env.BUILDHOST_PROJECT} as ${project} with ${inputAptDepends}`);\n")},
			// A `??` arm that goes on past its line stays.
			workflowCase("env-indirection-github-script", workflowHeader()+"jobs:\n  build:\n    runs-on: ubuntu-latest\n"+
				"    steps:\n      - uses: actions/github-script@v7\n        env:\n          SHA: ${{ github.sha }}\n"+
				"        with:\n          script: |\n            core.info(process.env.SHA);\n"+
				"            core.info(process.env.SHA ??\n              \"none\");\n"),
			envKeptCase(),
		},
	})
}

// scriptActionManifest is a composite action whose one step runs a TypeScript
// script with the given env block and script lines.
func scriptActionManifest(env, script string) string {
	return "name: Publish\ndescription: Publish a release.\n\ninputs:\n  project:\n    description: Project name\n    required: false\n" +
		"  apt_depends:\n    description: Runtime prerequisites\n    required: false\n\n" +
		"runs:\n  using: composite\n  steps:\n    - id: publish\n      uses: wow-look-at-my/actions@typescript#latest\n" +
		env + "      with:\n        script: |\n" + script
}

// envKeptCase holds entries with a job to do. A script that assigns the
// variable needs it in the environment. A name another step reads keeps its
// entry too.
func envKeptCase() RuleCase {
	c := RuleCase{Name: "yaml/env-indirection-typescript-kept", Path: ".github/actions/buildhost-publish/action.yml", Text: scriptActionManifest(
		"      env:\n        BUILDHOST_APT_DEPENDS: ${{ inputs.apt_depends }}\n        BUILDHOST_PROJECT: ${{ inputs.project }}\n",
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
