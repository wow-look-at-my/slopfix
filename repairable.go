package slopfix

import (
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// EveryID names every rule the registry holds, the repository rules included.
func EveryID() set.Set[string] {
	ids := set.New[string]()
	for _, rule := range AllRuleSpecs() {
		ids.Add(rule.ID)
	}
	return ids
}

// WarningIDs names every rule whose findings are warnings. A warning fails no check, so no repair answers it.
var WarningIDs = ste.WarningIDs.Union(workflow.WarningIDs)

// Repairable reports whether slopfix repairs the DEFECT a finding names,
// rather than the rule that found it.
func Repairable(id string) bool {
	spec, ok := RuleSpecByID(id)
	return ok && spec.Autofix != nil
}
