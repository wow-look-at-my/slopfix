package slopfix

import (
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// RuleSpec is one rule: what it detects, the repair that answers that
// detection, and the cases that prove both. A rule is registered as a whole,
// so a detection cannot enter the registry without an autofix or a declared
// report-only exemption.
//
// Detect answers the findings this rule reports for a case. Autofix rewrites
// the case's substrate and is required unless ReportOnly names why the rule
// reports without one. Cases are the rule's own worked examples, and the
// harness runs each through Detect, Autofix and Detect again.
type RuleSpec struct {
	// ID is the name a report prints and --only accepts.
	ID string
	// Category is the family --only names in full.
	Category Rule
	// Detect finds this rule's findings in a case.
	Detect func(RuleCase) []ste.Finding
	// Autofix rewrites the case so this rule no longer detects it. It is nil
	// only for a rule whose ReportOnly says why.
	Autofix func(RuleCase) RuleCase
	// Cases are the worked examples the harness drives.
	Cases []RuleCase
	// ReportOnly is the declared exemption for a rule no rewrite answers. It
	// is a reason in words, and it is empty for every rule that repairs.
	ReportOnly string
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
}

// A rule with a detection needs a repair or a declared exemption. The registry
// refuses anything else at init, so the build fails where the rule is added.
var (
	ruleMu       sync.Mutex
	ruleRegistry []RuleSpec
	ruleIndex    = map[string]RuleSpec{}
)

// RegisterRule adds a rule to the registry. It panics on a rule that carries a
// detection and neither an autofix nor a declared report-only exemption, on a
// rule with no case to prove it, and on a name taken twice. The panic fires at
// init, so an unfixed detection never reaches a build.
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
	if len(r.Cases) == 0 {
		panic(fmt.Sprintf("rule %q: the harness has no case to prove its detection", r.ID))
	}
	if r.Autofix == nil && r.ReportOnly == "" {
		panic(fmt.Sprintf("rule %q: a detection with no autofix and no declared report-only exemption", r.ID))
	}
	if r.Autofix != nil && r.ReportOnly != "" {
		panic(fmt.Sprintf("rule %q: it repairs, so it cannot also claim ReportOnly", r.ID))
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

// ruleCategoryOrder is the order the categories are listed in, so --only prints
// a stable help.
var ruleCategoryOrder = []Rule{RuleTombstones, RuleCounts, RuleWrap, RuleSTE, RuleEnglish, RuleComments, RuleWorkflow, RuleRepo, RulePins}

// ruleCategories answers the categories the registry carries, in listing order.
func ruleCategories() []Rule {
	present := set.New[Rule]()
	for _, r := range AllRuleSpecs() {
		present.Add(r.Category)
	}
	var out []Rule
	for _, c := range ruleCategoryOrder {
		if present.Contains(c) {
			out = append(out, c)
		}
	}
	for _, c := range present.Values() {
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
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

// --- detection and autofix adapters -----------------------------------------

// detectContent answers the findings this rule reports in one file's text. It
// unions what check reads with what a report keeps, so a rule that surfaces as
// a kept tombstone or a section finding is found too.
func detectContent(ids ...string) func(RuleCase) []ste.Finding {
	want := set.Of(ids...)
	return func(c RuleCase) []ste.Finding {
		var out []ste.Finding
		add := func(f ste.Finding) {
			if want.Contains(f.ID) {
				out = append(out, f)
			}
		}
		for _, f := range CheckContent(c.Path, c.Text) {
			add(f)
		}
		rep := Report(Request{Content: c.Text, Path: c.Path, MaxCommentLines: tombstones.DefaultMaxCommentLines})
		for _, f := range rep.Findings {
			add(f)
		}
		for _, k := range rep.Kept {
			if want.Contains(k.ID) {
				out = append(out, ste.Finding{Line: k.LineNo, ID: k.ID, Rule: k.Tell, Detail: k.Phrase, Fix: k.Fix})
			}
		}
		return out
	}
}

// repairContent answers the file's text with this rule's repair applied and
// every other rule left alone.
func repairContent(ids ...string) func(RuleCase) RuleCase {
	return func(c RuleCase) RuleCase {
		c.Text = Fix(Request{
			Content:         c.Text,
			Path:            c.Path,
			IDs:             ids,
			MaxCommentLines: tombstones.DefaultMaxCommentLines,
		}).Text
		return c
	}
}

// detectTree answers this rule's findings over a repository the tree rules
// judge. It reports findings and kept tombstones alike.
func detectTree(ids ...string) func(RuleCase) []ste.Finding {
	want := set.Of(ids...)
	return func(c RuleCase) []ste.Finding {
		out := CheckTreeWith(c.Root, Request{MaxCommentLines: tombstones.DefaultMaxCommentLines})
		var found []ste.Finding
		for _, f := range out.Findings {
			if want.Contains(f.ID) {
				found = append(found, f.Finding)
			}
		}
		for _, k := range out.Kept {
			if want.Contains(k.ID) {
				found = append(found, ste.Finding{Line: k.LineNo, ID: k.ID, Rule: k.Tell, Detail: k.Phrase, Fix: k.Fix})
			}
		}
		return found
	}
}

// repairTree answers the repository with this rule's repair applied.
func repairTree(ids ...string) func(RuleCase) RuleCase {
	return func(c RuleCase) RuleCase {
		FixTreeWith(c.Root, Request{IDs: ids, MaxCommentLines: tombstones.DefaultMaxCommentLines})
		return c
	}
}

// patternGroups indexes the english table's identified patterns by rule ID.
func patternGroups() map[string][]english.Pattern {
	groups := map[string][]english.Pattern{}
	for _, p := range english.Patterns() {
		if p.ID == "" {
			continue
		}
		groups[p.ID] = append(groups[p.ID], p)
	}
	return groups
}

// detectPattern answers the patterns a wording rule carries, reporting the rule
// when any of them rewrites the case's text.
func detectPattern(patterns []english.Pattern, id string) func(RuleCase) []ste.Finding {
	return func(c RuleCase) []ste.Finding {
		for _, p := range patterns {
			if _, took := p.ApplyN(c.Text); took > 0 {
				return []ste.Finding{{
					Line:   1,
					ID:     id,
					Rule:   "a phrase about a state the code has left",
					Detail: p.Match,
					Fix:    "Cut the phrase. `slopfix fix` does this.",
				}}
			}
		}
		return nil
	}
}

// repairPattern rewrites a wording rule's phrases out of the case's text.
func repairPattern(patterns []english.Pattern) func(RuleCase) RuleCase {
	return func(c RuleCase) RuleCase {
		for _, p := range patterns {
			c.Text = p.Apply(c.Text)
		}
		return c
	}
}

// patternCases answers the table's own worked examples for a wording rule.
func patternCases(patterns []english.Pattern, id string) []RuleCase {
	var out []RuleCase
	for n, p := range patterns {
		for m, t := range p.Tests() {
			if t.In == "" {
				continue
			}
			out = append(out, RuleCase{
				Name: fmt.Sprintf("%s-%d-%d", id, n, m),
				Path: "x.md",
				Text: t.In + "\n",
			})
		}
	}
	return out
}

// patternIDs answers every identified pattern ID the table carries, sorted, so
// the registry is built in a stable order.
func patternIDs() []string {
	ids := set.New[string]()
	for _, p := range english.Patterns() {
		if p.ID != "" {
			ids.Add(p.ID)
		}
	}
	out := slices.Sorted(ids.All())
	sort.Strings(out)
	return out
}
