package slopfix

import "github.com/wow-look-at-my/go-containers/set"

// EveryID names every rule the registry holds, the repository rules included.
func EveryID() set.Set[string] {
	ids := set.New[string]()
	for _, rule := range AllRuleSpecs() {
		ids.Add(rule.ID)
	}
	return ids
}

// Repairable reports whether slopfix repairs the DEFECT a finding names.
func Repairable(id string) bool {
	spec, ok := RuleSpecByID(id)
	return ok && spec.Autofix != nil
}
