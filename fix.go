package slopfix

import (
	"os"
	"slices"

	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// Rule names a repair Fix can apply. A caller that wants a single rule names it
// rather than reaching for a command of its own.
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
)

// AllRules is what Fix applies when a caller names none.
var AllRules = []Rule{RuleTombstones, RuleCounts, RuleWrap, RuleSTE}

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
	}
	return set.New[string]()
}

// Request is a piece of text put to Fix.
//
// Path names the file the text is headed for. It decides the comment syntax,
// and whether the prose rules apply at all. Empty means the caller vouches for
// the text as prose. Rules restricts what is applied, and empty means AllRules.
type Request struct {
	Content string
	Path    string
	Rules   []Rule
	// IDs restricts what is REPORTED to the rules named, the way a compiler
	// names a warning. Empty means every rule of every category in Rules.
	IDs []string
	// MaxCommentLines caps a comment block. A cap of nothing turns it off.
	MaxCommentLines int
}

// Repair is what a caller gets back for a piece of text: the text as this
// binary would write it, and what no rewrite can repair.
//
// A hook reads Text to replace the write it was about to allow, and Kept plus
// Findings to refuse it.
type Repair struct {
	// Text is the repaired text. It equals the input when Changed is false.
	Text string `json:"text"`
	// Changed reports whether any rewrite applied.
	Changed bool `json:"changed"`
	// Removed names each span the repair cut out.
	Removed []string `json:"removed,omitempty"`
	// Kept carries the tombstones no whole-line deletion resolves.
	Kept []tombstones.Hit `json:"kept,omitempty"`
	// Findings are what a reader must repair by hand.
	Findings []ste.Finding `json:"findings"`
}

// Fix repairs what a rewrite can repair and reports the rest.
//
// The repairs run in the order that keeps every span valid. A tombstone
// line leads, because it is deleted whole. The count strip follows, on
// text whose deletions have landed. The wrap join comes last, and the prose
// repair runs inside it, on each block's joined text, because a rule reads a
// paragraph as a single sentence stream and a hand wrap hides half of it.
//
// The join must only move newlines. Format proves that on this document ahead
// of everything else, and a document it cannot prove keeps its own line
// breaks. The prose repair is
// different in kind: it changes words on purpose, each to what Check names.
func Fix(req Request) Repair {
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

	text := req.Content
	var repair Repair

	if wants(RuleTombstones) && req.Path != "" {
		cut := tombstones.Fix(req.Path, text, req.MaxCommentLines)
		text = cut.Text
		repair.Removed = append(repair.Removed, cut.Removed...)
		for _, hit := range cut.Kept {
			if keeps(hit.ID) {
				repair.Kept = append(repair.Kept, hit)
			}
		}
	}

	// The remaining rules read prose. Source keeps its own text, because a
	// comma splice inside a code line is not a sentence.
	if req.Path != "" && !IsDocument(req.Path) {
		repair.Text = text
		repair.Changed = text != req.Content
		return repair
	}

	if wants(RuleCounts) && keeps(counts.ID) {
		stripped, hits := counts.Strip(text)
		text = stripped
		for _, hit := range hits {
			repair.Removed = append(repair.Removed, hit.Phrase)
		}
	}
	// The join and the word repair share a pass, because a rule reads a
	// paragraph as a sentence stream and a hand wrap hides half of it. Naming a
	// ste rule therefore rewraps the paragraph it repairs.
	joins := wants(RuleWrap) && keeps(IDHardWrap)
	prose := wants(RuleSTE)
	if joins || prose {
		word := func(text string) string { return text }
		if prose {
			word = func(text string) string { return ste.FixSelected(text, keeps) }
		}
		if _, safe := Format(text); safe {
			text = markdown.FormatFunc(text, word)
		}
	}
	if wants(RuleSTE) {
		for _, finding := range Check(text) {
			if keeps(finding.ID) {
				repair.Findings = append(repair.Findings, finding)
			}
		}
	}

	repair.Text = text
	repair.Changed = text != req.Content
	return repair
}

// IsDocument reports whether path names prose rather than source.
func IsDocument(path string) bool { return tombstones.IsDocument(path) }

// FixFile repairs a file in place and reports what it did. It writes nothing
// when the repair leaves the document as it was.
func FixFile(path string) (Repair, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Repair{}, err
	}
	repair := Fix(Request{Content: string(content), Path: path})
	if !repair.Changed {
		return repair, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return repair, err
	}
	if err := os.WriteFile(path, []byte(repair.Text), info.Mode().Perm()); err != nil {
		return repair, err
	}
	return repair, nil
}
