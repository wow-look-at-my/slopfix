package slopfmt

import (
	"os"

	"github.com/wow-look-at-my/slopfmt/counts"
	"github.com/wow-look-at-my/slopfmt/markdown"
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
// Three repairs run, in this order. The count strip goes first, on the source,
// so a hit's byte span is still valid. The wrap join follows. The prose repair
// runs inside the join, on each block's joined text, because a rule reads a
// paragraph as one sentence stream and a hand wrap hides half of it.
//
// The join must only move newlines. Format proves that on this document first,
// and a document it cannot prove is returned with its counts cut and nothing
// else. The prose repair is different in kind: it changes words on purpose,
// each one to the replacement Check names.
func Fix(content string) Repair {
	stripped, cut := counts.Strip(content)
	removed := make([]string, 0, len(cut))
	for _, hit := range cut {
		removed = append(removed, hit.Phrase)
	}

	repaired := stripped
	if _, safe := Format(stripped); safe {
		repaired = markdown.FormatFunc(stripped, ste.Fix)
	}
	return Repair{
		Text:     repaired,
		Changed:  repaired != content,
		Removed:  removed,
		Findings: Check(repaired),
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

// FixFile repairs a file in place and reports what it did. It writes nothing
// when the repair leaves the document as it was.
func FixFile(path string) (Repair, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Repair{}, err
	}
	repair := Fix(string(content))
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
