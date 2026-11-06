// The workflow fixtures the yaml rules are driven on.

package slopfix

import _ "embed"

// The workflow fixtures the yaml rules are driven on. Each rule file names
// the one it needs.

func workflowCase(name, body string) RuleCase {
	return RuleCase{Name: "yaml/" + name, Path: ".github/workflows/ci.yml", Text: body}
}

func workflowHeader() string {
	return workflowHeaderYml + workflowConcurrency
}

//go:embed testdata/workflow-concurrency.yml
var workflowConcurrency string

func workflowTail() string {
	return workflowTailYml
}

//go:embed testdata/workflow-header.yml
var workflowHeaderYml string

//go:embed testdata/workflow-tail.yml
var workflowTailYml string
