package slopfix

import (
	"fmt"
	"slices"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// The shared reads a rule's own detect and autofix functions are written on.
// Each rule file names its own pair and registers itself, so no file holds a
// list of the rules. A rule file that is deleted takes its rule with it.
//
// These are the plumbing, not a rule: they answer what one rule's ID reports on
// a case, and how the fix pipeline rewrites that ID.

// caseFindings answers the findings one rule reports in a file case. It unions
// what check reads with what a report keeps, so a kept tombstone or a section
// finding surfaces too.
func caseFindings(c RuleCase, id string) []ste.Finding {
	var out []ste.Finding
	keep := func(f ste.Finding) {
		if f.ID == id {
			out = append(out, f)
		}
	}
	for _, f := range CheckContent(c.Path, c.Text) {
		keep(f)
	}
	rep := Report(Request{Content: c.Text, Path: c.Path, MaxCommentLines: tombstones.DefaultMaxCommentLines})
	for _, f := range rep.Findings {
		keep(f)
	}
	for _, k := range rep.Kept {
		if k.ID == id {
			out = append(out, ste.Finding{Line: k.LineNo, ID: k.ID, Rule: k.Tell, Detail: k.Phrase, Fix: k.Fix})
		}
	}
	return out
}

// caseAutofix answers a file case with one rule's repair applied and every
// other rule left alone.
func caseAutofix(c RuleCase, id string) RuleCase {
	c.Text = Fix(Request{
		Content:         c.Text,
		Path:            c.Path,
		IDs:             []string{id},
		MaxCommentLines: tombstones.DefaultMaxCommentLines,
	}).Text
	return c
}

// treeFindings answers the findings one tree rule reports over a repository.
func treeFindings(c RuleCase, id string) []ste.Finding {
	out := CheckTreeWith(c.Root, Request{MaxCommentLines: tombstones.DefaultMaxCommentLines})
	var found []ste.Finding
	for _, f := range out.Findings {
		if f.ID == id {
			found = append(found, f.Finding)
		}
	}
	for _, k := range out.Kept {
		if k.ID == id {
			found = append(found, ste.Finding{Line: k.LineNo, ID: k.ID, Rule: k.Tell, Detail: k.Phrase, Fix: k.Fix})
		}
	}
	return found
}

// treeAutofix answers a repository with one tree rule's repair applied.
func treeAutofix(c RuleCase, id string) RuleCase {
	FixTreeWith(c.Root, Request{IDs: []string{id}, MaxCommentLines: tombstones.DefaultMaxCommentLines})
	return c
}

// AllRules is every category the registry uses, in the order a rule first
// declared it. The registry is filled by the rules' own init functions, so the
// list is read from the registry rather than written down.
func AllRules() []Rule {
	var out []Rule
	seen := set.New[Rule]()
	for _, r := range AllRuleSpecs() {
		if seen.Contains(r.Category) {
			continue
		}
		seen.Add(r.Category)
		out = append(out, r.Category)
	}
	return out
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

// repeatedPhrase writes phrase enough times to reach a length, for a fixture
// that has to cross a cap.
func repeatedPhrase(phrase string, times int) string {
	out := ""
	for range times {
		out += phrase
	}
	return out
}

// sortedIDs answers the IDs of a set, in report order.
func sortedIDs(ids set.Set[string]) []string { return slices.Sorted(ids.All()) }
