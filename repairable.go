package slopfix

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// EveryID names every rule a category holds, the repository rules included.
func EveryID() set.Set[string] {
	ids := set.New[string]()
	for _, rule := range AllRules {
		for id := range IDsFor(rule).All() {
			ids.Add(id)
		}
	}
	return ids
}

// WarningIDs names every rule whose findings are warnings. A warning fails no check, so no repair answers it.
var WarningIDs = ste.WarningIDs.Union(workflow.WarningIDs)

// Repairable reports whether slopfix repairs the DEFECT a finding names,
// rather than the rule that found it. A rule repairs when it is registered
// with an autofix. A rule that declares itself report-only does not.
func Repairable(id string) bool {
	spec, ok := RuleSpecByID(id)
	return ok && spec.Autofix != nil
}
