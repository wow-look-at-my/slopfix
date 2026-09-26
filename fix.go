package slopfix

import (
	"fmt"
	"os"
	"slices"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/trace"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// Rule names a repair Fix can apply, so a caller can select a repair by name.
type Rule string

const (
	// RuleTombstones strips a comment about a state the code has left.
	RuleTombstones Rule = "tombstones"
	// RuleCounts cuts the cardinal out of an inventory count.
	RuleCounts Rule = "counts"
	// RuleWrap joins a hand-wrapped paragraph back into a line.
	RuleWrap Rule = "wrap"
	// RuleSTE reports what fails the merge gate and repairs nothing.
	RuleSTE Rule = "ste"
	// RuleComments is a block that fits its code, and a number said in words.
	RuleComments Rule = "comments"
	// RuleWorkflow is what a workflow owes the gate it runs.
	RuleWorkflow Rule = "yaml"
)

// AllRules is what Fix applies when a caller names none.
var AllRules = []Rule{RuleTombstones, RuleCounts, RuleWrap, RuleSTE, RuleComments, RuleWorkflow, RuleRepo}

// IDsFor names every rule inside a category, so a caller can reject a typo
// before it applies nothing and reads as a clean file.
func IDsFor(rule Rule) set.Set[string] {
	switch rule {
	case RuleTombstones:
		return tombstones.AllIDs()
	case RuleCounts:
		return set.Of(counts.ID)
	case RuleWrap:
		return set.Of(IDHardWrap)
	case RuleSTE:
		return ste.AllIDs
	case RuleComments:
		return set.Of(commentfix.IDLength, commentfix.ID, commentfix.IDTail)
	case RuleWorkflow:
		return workflow.AllIDs
	case RuleRepo:
		return RepoIDs
	}
	return set.New[string]()
}

// Request is a piece of text put to Fix. Path decides the comment syntax, and
// an empty Rules means AllRules.
type Request struct {
	Content string
	Path    string
	Rules   []Rule
	// IDs restricts what is REPORTED. Empty means every rule in Rules.
	IDs []string
	// MaxCommentLines caps a comment block. A cap of nothing turns it off.
	MaxCommentLines int
	// Scope bounds where a repair may land. The zero Scope is the whole file.
	Scope edit.Scope `json:"-"`
}

// Repair is the text as this binary would write it, plus what the rewrite flagged.
type Repair struct {
	// Text is the repaired text. It equals the input when Changed is false.
	Text string `json:"text"`
	// Changed reports whether any rewrite applied.
	Changed bool `json:"changed"`
	// Removed names each span the repair cut out.
	Removed []string `json:"removed,omitempty"`
	// Rewrites counts the prose repairs the english table had to make.
	Rewrites int `json:"rewrites,omitempty"`
	// Kept carries the tombstones no whole-line deletion resolves.
	Kept []tombstones.Hit `json:"kept,omitempty"`
	// Findings are what a reader must repair by hand.
	Findings []ste.Finding `json:"findings"`
	// Refused quotes each rewrite a parser would not let land, and why.
	Refused []string `json:"refused,omitempty"`
	// Scope bounds, in Text, what the Request's Scope bounded.
	Scope edit.Scope `json:"-"`
}

// refuse records the edits a gate would not write.
func (r *Repair) refuse(refused []edit.Refused) {
	for _, x := range refused {
		r.Refused = append(r.Refused, fmt.Sprintf("%s: %q", x.Reason, x.Edit.Text))
	}
}

// Fix repairs what a rewrite can repair and reports the rest. The repairs run
// in an order that keeps every span valid. The join only moves newlines.
func Fix(req Request) Repair {
	defer trace.Phase("fix/all")()
	rules := req.Rules
	if len(rules) == 0 {
		rules = AllRules
	}
	wants := func(r Rule) bool { return slices.Contains(rules, r) }
	// An ID names a rule inside a category, the way a compiler names a
	// warning. Naming any turns the others off, and naming none keeps them all.
	keeps := func(id string) bool {
		return len(req.IDs) == 0 || slices.Contains(req.IDs, id)
	}

	// Every repair is a registered fixer, and every fixer writes through the
	// gate the file's parser owns. The kind picks both the fixers and the gate.
	kind := kindOf(req.Path, req.Content)
	opts := fixer.Options{
		Kind:            kind,
		Scope:           req.Scope,
		Wants:           func(c string) bool { return wants(Rule(c)) },
		Keeps:           keeps,
		MaxCommentLines: req.MaxCommentLines,
	}
	if kind == fixer.Workflow {
		opts = workflow.Options(opts)
	}
	f := fixer.Open(req.Path, req.Content, opts)
	fixer.Run(f, fixer.For(kind))

	text := f.Text()
	rep := f.Report()
	repair := Repair{Text: text, Changed: text != req.Content, Removed: rep.Removed, Rewrites: rep.Rewrites, Scope: f.Scope()}
	repair.refuse(rep.Refused)
	for _, note := range rep.Notes {
		if hit, ok := note.(tombstones.Hit); ok && keeps(hit.ID) {
			repair.Kept = append(repair.Kept, hit)
		}
	}

	switch kind {
	case fixer.Workflow:
		for _, finding := range workflow.Check(text) {
			if keeps(finding.ID) {
				repair.Findings = append(repair.Findings, finding)
			}
		}
	case fixer.Source:
		// Source keeps its own text for the prose rules, because a comma splice
		// inside a code line is not a sentence.
		if wants(RuleComments) && keeps(commentfix.IDLength) {
			for _, hit := range commentfix.CheckLength(req.Path, text) {
				repair.Kept = append(repair.Kept, tombstones.Hit{
					ID:     hit.ID,
					Tell:   hit.Tell,
					Phrase: hit.Sentence,
					LineNo: hit.Line,
				})
			}
		}
	case fixer.Document:
		if wants(RuleSTE) {
			for _, finding := range Check(text) {
				if keeps(finding.ID) {
					repair.Findings = append(repair.Findings, finding)
				}
			}
		}
	}
	return repair
}

// kindOf answers which parser owns a file. An empty path is prose: a caller
// holding text and naming no file is asking about prose, not about a tree.
func kindOf(path, content string) fixer.Kind {
	switch {
	case path != "" && isWorkflow(path, content):
		return fixer.Workflow
	case path != "" && !IsDocument(path):
		return fixer.Source
	}
	return fixer.Document
}

// The join and the word repair share a pass: the formatter rewrites each.
func init() {
	fixer.Register(fixer.Spec{
		Label:    "wrap-and-ste",
		Families: []string{string(RuleWrap), string(RuleSTE)},
		Rules:    append([]string{IDHardWrap}, slices.Sorted(ste.AllIDs.All())...),
		Files:    []fixer.Kind{fixer.Document},
		Place:    30,
		Repair: func(f *fixer.File) {
			defer trace.Phase("fix/wrap-and-ste")()
			word := func(text string) string { return text }
			if f.Wants(string(RuleSTE)) {
				word = func(text string) string { return ste.FixSelected(text, f.Keeps) }
			}
			if _, safe := Format(f.Text()); safe {
				f.Apply(markdown.FormatEdits(f.Text(), word))
			}
		},
	})
}

// IsDocument reports whether path names prose rather than source.
func IsDocument(path string) bool { return tombstones.IsDocument(path) }

// FixFile repairs a file in place under every rule.
func FixFile(path string) (Repair, error) {
	return FixFileWith(path, Request{})
}

// FixFileWith repairs a file in place under the caller's own selection, and
// reports what it did.
//
// The Content and Path of req are the file's, whatever the caller put there.
// Everything else is the caller's: a run that names a rule on the command line
// has to reach the repair, or the selection is silently ignored.
func FixFileWith(path string, req Request) (Repair, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Repair{}, err
	}
	req.Content, req.Path = string(content), path
	repair := Fix(req)
	if !repair.Changed {
		return repair, nil
	}
	return repair, commentfix.WriteFile(path, repair.Text)
}
