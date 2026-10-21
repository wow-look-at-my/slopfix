package slopfix

import _ "embed"

import "github.com/wow-look-at-my/slopfix/ste"

// repo/package-scripts: a package.json whose scripts run a command wrapper the
// repository does not need. The repair writes the command the wrapper stood for.
func init() {
	RegisterRule(RuleSpec{
		ID:       IDPackageScripts,
		Category: RuleRepo,
		Detect:   detectPackageScripts,
		Autofix:  autofixPackageScripts,
		Cases: []RuleCase{{Name: IDPackageScripts, Files: map[string]string{
			"package.json": embeddedJson,
		}}},
	})
}

// detectPackageScripts answers every script the rule reports.
func detectPackageScripts(c RuleCase) []ste.Finding { return treeFindings(c, IDPackageScripts) }

// autofixPackageScripts writes the command a script wrapper stood for.
func autofixPackageScripts(c RuleCase) RuleCase { return treeAutofix(c, IDPackageScripts) }

//go:embed testdata/embedded.json
var embeddedJson string
