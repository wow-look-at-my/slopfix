package slopfix

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
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

// RuleCase is a fixture: a repository and a closing message.
type RuleCase struct {
	// Name labels the case in a failure.
	Name string
	// Path names a single file of the repository, and Text is its content. With no Path, Text is the closing message.
	Path string
	Text string
	// Files are the other files of the repository, by path from its root.
	Files map[string]string
	// Root is the directory Materialize wrote the repository under.
	Root string
}

// Materialize writes the case's repository under dir with the marker the
// repository rules read, and answers the case rooted there. Afterwards Path is
// empty and Text holds the message alone, so every rule reads one shape.
func Materialize(dir string, c RuleCase) (RuleCase, error) {
	files := maps.Clone(c.Files)
	if files == nil {
		files = map[string]string{}
	}
	if c.Path != "" {
		files[c.Path] = c.Text
		c.Path, c.Text = "", ""
	}
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		return c, err
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return c, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return c, err
		}
	}
	c.Files, c.Root = nil, dir
	return c, nil
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
