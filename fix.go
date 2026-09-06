package slopfmt

import (
	"slices"

	"github.com/wow-look-at-my/slopfmt/counts"
	"github.com/wow-look-at-my/slopfmt/ste"
	"github.com/wow-look-at-my/slopfmt/tombstones"
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

// Request is a piece of text put to Fix.
//
// Path names the file the text is headed for. It decides the comment syntax,
// and whether the prose rules apply at all. Empty means the caller vouches for
// the text as prose. Rules restricts what is applied, and empty means AllRules.
type Request struct {
	Content string
	Path    string
	Rules   []Rule
	// MaxCommentLines caps a comment block. Zero turns the cap off.
	MaxCommentLines int
}

// Repair is what a caller gets back for a piece of text: the text as this
// binary would write it, and what no rewrite can repair.
//
// A hook reads Text to replace the write it was about to allow, and Kept plus
// Findings to refuse one.
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
// The rules run in the order that keeps each one's spans valid: tombstone lines
// go first, then counts, then the wrap join, which reads text whose cuts have
// already landed. A join that changed a word is dropped, because joining must
// only move newlines.
func Fix(req Request) Repair {
	rules := req.Rules
	if len(rules) == 0 {
		rules = AllRules
	}
	wants := func(r Rule) bool { return slices.Contains(rules, r) }

	text := req.Content
	var repair Repair

	if wants(RuleTombstones) && req.Path != "" {
		cut := tombstones.Fix(req.Path, text, req.MaxCommentLines)
		text = cut.Text
		repair.Removed = append(repair.Removed, cut.Removed...)
		repair.Kept = cut.Kept
	}

	// The remaining rules read prose. Source is left alone, because a comma
	// splice inside a code line is not a sentence.
	if req.Path != "" && !tombstones.IsDocument(req.Path) {
		repair.Text = text
		repair.Changed = text != req.Content
		return repair
	}

	if wants(RuleCounts) {
		stripped, hits := counts.Strip(text)
		text = stripped
		for _, hit := range hits {
			repair.Removed = append(repair.Removed, hit.Phrase)
		}
	}
	if wants(RuleWrap) {
		if formatted, safe := Format(text); safe {
			text = formatted
		}
	}
	if wants(RuleSTE) {
		repair.Findings = Check(text)
	}

	repair.Text = text
	repair.Changed = text != req.Content
	return repair
}
