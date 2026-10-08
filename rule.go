package slopfix

import (
	"fmt"
	"slices"
	"sync"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
)

// RuleSpec is one rule: what it detects, the repair that answers that
// detection, and the cases that prove both.
type RuleSpec struct {
	// ID is the name a report prints and --only accepts.
	ID string
	// Category is the family --only names in full.
	Category Rule
	// Detect finds this rule's findings in a case.
	Detect func(RuleCase) []ste.Finding
	// Autofix rewrites the case so this rule no longer detects it.
	Autofix func(RuleCase) RuleCase
	// Cases are the worked examples the harness drives.
	Cases []RuleCase
}

// RuleCase is the substrate a rule reads: a file's path and text, or a
// repository the tree rules judge through the files under Root.
type RuleCase struct {
	// Name labels the case in a failure.
	Name string
	// Path is the file the text is headed for, and decides the parser.
	Path string
	// Text is a file case's content.
	Text string
	// Root is a tree case's directory. The harness fills it from Files.
	Root string
	// Files are written under Root before a tree case runs.
	Files map[string]string
	// Unchanged marks a case the rule must leave as written.
	Unchanged bool
}

// A rule with a detection needs an autofix. There is no exemption.
var (
	ruleMu       sync.Mutex
	ruleRegistry []RuleSpec
	ruleIndex    = map[string]RuleSpec{}
)

// RegisterRule adds a rule to the registry. The build panics at init when a
// rule is malformed, when its Autofix is nil, or when its name is taken twice.
func RegisterRule(r RuleSpec) {
	if r.ID == "" {
		panic("rule: a rule carries an empty ID")
	}
	if r.Category == "" {
		panic(fmt.Sprintf("rule %q: no category, so --only cannot select it", r.ID))
	}
	if r.Detect == nil {
		panic(fmt.Sprintf("rule %q: a detection is required", r.ID))
	}
	if r.Autofix == nil {
		panic(fmt.Sprintf("rule %q: a detection with no autofix is a finding handed to the reader with no way out of it", r.ID))
	}
	if len(r.Cases) == 0 {
		panic(fmt.Sprintf("rule %q: the harness has no case to prove its detection", r.ID))
	}
	ruleMu.Lock()
	defer ruleMu.Unlock()
	if _, taken := ruleIndex[r.ID]; taken {
		panic(fmt.Sprintf("rule %q is registered twice", r.ID))
	}
	ruleRegistry = append(ruleRegistry, r)
	ruleIndex[r.ID] = r
}

// AllRuleSpecs answers every registered rule, in the order it was registered.
func AllRuleSpecs() []RuleSpec {
	ruleMu.Lock()
	defer ruleMu.Unlock()
	return slices.Clone(ruleRegistry)
}

// RuleSpecByID answers a registered rule, and whether the name is known.
func RuleSpecByID(id string) (RuleSpec, bool) {
	ruleMu.Lock()
	defer ruleMu.Unlock()
	r, ok := ruleIndex[id]
	return r, ok
}

// ruleIDsIn answers the IDs a category holds, as a set.
func ruleIDsIn(category Rule) set.Set[string] {
	ids := set.New[string]()
	for _, r := range AllRuleSpecs() {
		if r.Category == category {
			ids.Add(r.ID)
		}
	}
	return ids
}
