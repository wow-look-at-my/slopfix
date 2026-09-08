package slopfix

import (
	"os"
	"slices"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentlength"
	"github.com/wow-look-at-my/slopfix/commentnumbers"
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
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
	// RuleComments is what a comment owes its code: a block that fits inside
	// it, and a number said in words rather than stated.
	RuleComments Rule = "comments"
)

// AllRules is what Fix applies when a caller names none.
var AllRules = []Rule{RuleTombstones, RuleCounts, RuleWrap, RuleSTE, RuleComments}

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
		return set.Of(commentlength.ID, commentnumbers.ID)
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
}

// Repair is the text as this binary would write it, plus what no rewrite can repair.
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

// Fix repairs what a rewrite can repair and reports the rest. The repairs run
// in an order that keeps every span valid. The join only moves newlines.
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

	// Naming an ID turns the other rules off, and the strip below deletes whole
	// lines: without this guard, `--only comments/number` cuts a line the
	// tombstone rule judged, which is another rule's repair applied unasked.
	if wants(RuleTombstones) && req.Path != "" && anyKept(tombstones.AllIDs(), keeps) {
		cut := tombstones.Fix(req.Path, text, req.MaxCommentLines)
		text = cut.Text
		repair.Removed = append(repair.Removed, cut.Removed...)
		for _, hit := range cut.Kept {
			if keeps(hit.ID) {
				repair.Kept = append(repair.Kept, hit)
			}
		}
	}

	// The comment-length repair reads source rather than prose, so it runs
	// before the document gate below sends a source file home.
	if wants(RuleComments) && keeps(commentlength.ID) && req.Path != "" {
		cut, changed := commentlength.Fix(req.Path, text)
		if changed {
			text = cut
			repair.Removed = append(repair.Removed, "trailing comment prose")
		}
		for _, hit := range commentlength.Check(req.Path, text) {
			repair.Kept = append(repair.Kept, tombstones.Hit{
				ID:     hit.ID,
				Tell:   hit.Tell,
				Phrase: hit.Sentence,
				LineNo: hit.Line,
			})
		}
	}

	// The number repair reads source too, and runs after the length cut: a
	// sentence the cut already took is a sentence this one need not rewrite.
	if wants(RuleComments) && keeps(commentnumbers.ID) && req.Path != "" {
		said := commentnumbers.Fix(req.Path, text)
		if said.Changed {
			text = said.Text
			repair.Removed = append(repair.Removed, said.Removed...)
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

// anyKept reports whether the caller's ID selection keeps any rule of a set.
func anyKept(ids set.Set[string], keeps func(string) bool) bool {
	for id := range ids.All() {
		if keeps(id) {
			return true
		}
	}
	return false
}

// IsDocument reports whether path names prose rather than source.
func IsDocument(path string) bool { return tombstones.IsDocument(path) }

// FixFile repairs a file in place under every rule.
func FixFile(path string) (Repair, error) {
	return FixFileWith(path, Request{})
}

// FixFileWith repairs a file in place under the caller's own selection, and
// reports what it did. It writes nothing when the repair leaves the file as it
// was.
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
	info, err := os.Stat(path)
	if err != nil {
		return repair, err
	}
	if err := os.WriteFile(path, []byte(repair.Text), info.Mode().Perm()); err != nil {
		return repair, err
	}
	return repair, nil
}
