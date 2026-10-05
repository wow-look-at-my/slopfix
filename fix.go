package slopfix

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/counts"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/expect"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/forkscope"
	"github.com/wow-look-at-my/slopfix/goformat"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/pins"
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
	// RuleEnglish is plain English usage that STE does not cover.
	RuleEnglish Rule = "english"
	// RuleComments is a block that fits its code, and a number said in words.
	RuleComments Rule = "comments"
	// RuleWorkflow is what a workflow owes the gate it runs.
	RuleWorkflow Rule = "yaml"
	// RulePins is a download URL that names an exact release.
	RulePins Rule = "pins"
)

// AllRules is every category the registry uses, in the order a rule declared it.
var AllRules []Rule

// IDsFor names every rule inside a category, so a caller can reject a typo
// before it applies nothing and reads as a clean file.
func IDsFor(rule Rule) set.Set[string] {
	return ruleIDsIn(rule)
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
	// Owned names the lines of Content a fork wrote. A repair lands only on them, and a finding counts only on them.
	Owned *forkscope.Scope `json:"-"`
	// Fork names the lines a fork wrote across a tree run. A file it holds no line of is neither read nor written.
	Fork *forkscope.Lines `json:"-"`
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
	// Unmet names each slopfix-expect annotation the repair disagrees with.
	Unmet []string `json:"unmet,omitempty"`
}

// Fix repairs req, unless it carries slopfix-expect annotations.
func Fix(req Request) Repair {
	repair := within(req, fixAll(req))
	wide := widened(req)
	if wide == nil || wide == req.Owned || wide.All() || req.Scope.Bounded {
		return repair
	}
	// Each paragraph the fork wrote into is repaired as a text of its own and put back.
	text := repair.Text
	ranges := forkscope.Blocks(text, forkscope.Carry(req.Content, text, req.Owned))
	for i := len(ranges) - 1; i >= 0; i-- {
		var removed []string
		text, removed = fixBlock(req, text, ranges[i])
		repair.Removed = append(repair.Removed, removed...)
	}
	repair.Text, repair.Changed = text, text != req.Content
	owned := forkscope.Carry(req.Content, text, wide)
	landed := req
	landed.Content, landed.Owned, landed.Scope = text, nil, edit.Nowhere()
	after := Fix(landed)
	repair.Findings, repair.Kept = after.Findings, after.Kept
	repair = scoped(repair, forkscope.Carry(req.Content, text, req.Owned), owned)
	return upstreamRuns(req.Owned, repair.Text, repair)
}

// fixBlock repairs rows first to last of text with every rule but the run
// rules, and puts the result back in place.
func fixBlock(req Request, text string, rows [2]int) (string, []string) {
	lines := strings.SplitAfter(text, "\n")
	chunk := strings.Join(lines[rows[0]-1:rows[1]], "")
	one := req
	one.Content, one.Owned, one.Scope, one.IDs = chunk, nil, edit.Scope{}, blockFree(req)
	fixed := fixAll(one)
	out := fixed.Text
	if strings.HasSuffix(chunk, "\n") && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return strings.Join(lines[:rows[0]-1], "") + out + strings.Join(lines[rows[1]:], ""), fixed.Removed
}

// widened answers req.Owned grown to each whole block it reaches into.
func widened(req Request) *forkscope.Scope {
	if req.Owned == nil {
		return nil
	}
	// A source comment rule rewrites a whole run, so a run keeps to the fork's lines.
	if kindOf(req.Path, req.Content) != fixer.Document {
		return req.Owned
	}
	return req.Owned.Widen(req.Content)
}

// runRule reports whether rule id rewrites a whole comment run. Such a rule
// keeps to the lines the fork wrote, in a report and in a repair.
func runRule(id string) bool {
	return blockRules.Contains(id) || id == commentfix.IDLength || strings.HasPrefix(id, "tombstones/")
}

// scoped keeps each finding of repair on an owned line. A run rule reads
// narrow, and every other rule reads wide.
func scoped(repair Repair, narrow, wide *forkscope.Scope) Repair {
	var findings []ste.Finding
	for _, f := range repair.Findings {
		owned := wide
		if runRule(f.ID) {
			owned = narrow
		}
		if owned.Holds(f.Line, f.EndLine) {
			findings = append(findings, f)
		}
	}
	repair.Findings = findings
	repair.Kept = ownedHits(repair.Kept, narrow)
	return repair
}

// blockFree answers the rule IDs req selects, less each run rule.
func blockFree(req Request) []string {
	ids := req.IDs
	if len(ids) == 0 {
		ids = slices.Sorted(EveryRuleID().All())
	}
	var out []string
	for _, id := range ids {
		if !runRule(id) {
			out = append(out, id)
		}
	}
	return out
}

// fixAll is Fix with no regard to the lines a fork wrote.
func fixAll(req Request) Repair {
	if exempt(req.Path, req.Content) {
		return Repair{Text: req.Content, Scope: req.Scope}
	}
	notes := expect.Parse(req.Content)
	if !notes.Any() {
		return fixText(req)
	}
	original, scope := req.Content, req.Scope
	req.Content = notes.Stripped
	if scope.Bounded {
		req.Scope = edit.Within(notes.Shift(scope.Start), notes.Shift(scope.End))
	}
	repair := fixText(req)
	repair.Unmet = notes.Check(repair.Text)
	repair.Text, repair.Changed, repair.Scope = original, false, scope
	return repair
}

// exempt reports a file no rule reads or rewrites: another project's, or a
// generator's.
func exempt(path, content string) bool {
	return tombstones.Borrowed(path) || commentfix.IsGenerated(path, content)
}

// UnmetError is a fixture whose slopfix-expect annotations the repair broke.
type UnmetError struct {
	Path  string
	Unmet []string
}

func (e *UnmetError) Error() string {
	return fmt.Sprintf("%s: slopfix-expect does not hold:\n  %s", e.Path, strings.Join(e.Unmet, "\n  "))
}

// refuse records the edits a gate would not write.
func (r *Repair) refuse(refused []edit.Refused) {
	for _, x := range refused {
		r.Refused = append(r.Refused, fmt.Sprintf("%s: %q", x.Reason, x.Edit.Text))
	}
}

// fixText repairs what a rewrite can repair and reports the rest. The repairs
// run in an order that keeps every span valid. The join only moves newlines.
func fixText(req Request) Repair {
	defer trace.Phase("fix/all")()
	wants := wantsOf(req)
	keeps := keepsOf(req)

	// Every repair is a registered fixer, and every fixer writes through the gate the file's parser owns.
	kind := kindOf(req.Path, req.Content)
	f := openFile(req, kind)
	// A repair can hand a later rule new text, such as a division that leaves a count, so the passes run until the text holds.
	for range fixRounds {
		before := f.Text()
		fixer.Run(f, fixer.For(kind))
		if f.Text() == before {
			break
		}
	}

	text := f.Text()
	rep := f.Report()
	repair := Repair{Text: text, Changed: text != req.Content, Removed: rep.Removed, Rewrites: rep.Rewrites, Scope: f.Scope()}
	repair.refuse(rep.Refused)
	// A URL is text in every kind of file, so this rule reads the whole file.
	if wants(RulePins) {
		for _, finding := range pins.CheckPath(req.Path, text) {
			if keeps(finding.ID) {
				repair.Findings = append(repair.Findings, finding)
			}
		}
	}
	for _, note := range rep.Notes {
		if hit, ok := note.(tombstones.Hit); ok && keeps(hit.ID) {
			repair.Kept = append(repair.Kept, hit)
		}
	}

	switch kind {
	case fixer.Workflow:
		// A long sentence in a workflow comment is an STE rule, so --only ste
		// reaches it even when the run omits the yaml category.
		if wants(RuleSTE) && keeps(ste.IDSentenceCap) {
			repair.Findings = append(repair.Findings, sentenceFindings(req.Path, text)...)
		}
		if !wants(RuleWorkflow) {
			break
		}
		for _, finding := range workflow.Check(text) {
			if keeps(finding.ID) {
				repair.Findings = append(repair.Findings, finding)
			}
		}
	case fixer.Source:
		// Source keeps its own text for the prose rules, because a comma splice
		// inside a code line is not a sentence.
		// The number and tail rules report what their repair leaves, so check names what fix rewrites.
		for _, finding := range commentFindings(req.Path, text) {
			// The sentence cap is an STE rule, so the STE selection reaches it in a comment too.
			family := RuleComments
			if finding.ID == ste.IDSentenceCap {
				family = RuleSTE
			}
			if wants(family) && finding.ID != commentfix.IDLength && keeps(finding.ID) {
				repair.Findings = append(repair.Findings, finding)
			}
		}
		if wants(RuleComments) && keeps(commentfix.IDLength) {
			for _, hit := range commentfix.CheckLength(req.Path, text) {
				kept := tombstones.Hit{
					ID:     hit.ID,
					Tell:   hit.Tell,
					Phrase: hit.Sentence,
					LineNo: hit.Line,
				}
				if !hit.Repairable {
					kept.Fix = commentfix.FixLengthByHand
				}
				repair.Kept = append(repair.Kept, kept)
			}
		}
	case fixer.Document:
		if wants(RuleCounts) && keeps(counts.IDSection) {
			repair.Findings = append(repair.Findings, counts.SectionFindings(req.Path, text)...)
		}
		if wants(RuleSTE) || wants(RuleEnglish) {
			for _, finding := range Check(text) {
				if keeps(finding.ID) {
					repair.Findings = append(repair.Findings, finding)
				}
			}
		}
	}
	return repair
}

// fixRounds bounds the passes Fix runs, so repairs that undo each other stop.
const fixRounds = 4

// wantsOf answers the caller's category selection as a test. An empty Rules
// means AllRules.
func wantsOf(req Request) func(Rule) bool {
	rules := req.Rules
	if len(rules) == 0 {
		rules = AllRules
	}
	return func(r Rule) bool { return slices.Contains(rules, r) }
}

// openFile opens req for repair under the caller's selection, through the
// gate the kind's parser owns.
func openFile(req Request, kind fixer.Kind) *fixer.File {
	wants := wantsOf(req)
	opts := fixer.Options{
		Kind:            kind,
		Scope:           req.Scope,
		Wants:           func(c string) bool { return wants(Rule(c)) },
		Keeps:           keepsOf(req),
		MaxCommentLines: req.MaxCommentLines,
	}
	if kind == fixer.Workflow {
		opts = workflow.Options(opts)
	}
	return fixer.Open(req.Path, req.Content, opts)
}

// Report is Fix for a caller that writes nothing. No repair lands, so every
// finding is reported on the text as it stands, the repairable ones too.
func Report(req Request) Repair {
	repair := reportAll(req)
	if req.Owned == nil {
		return repair
	}
	repair = scoped(repair, req.Owned, widened(req))
	return upstreamRuns(req.Owned, req.Content, repair)
}

// reportAll is Report with no regard to the lines a fork wrote.
func reportAll(req Request) Repair {
	req.Owned = nil
	if !req.Scope.Bounded {
		req.Scope = edit.Nowhere()
	}
	repair := Fix(req)
	if exempt(req.Path, req.Content) {
		return repair
	}
	repair.Kept = append(repair.Kept, pending(req, repair.Kept)...)
	if len(req.Rules) > 0 && !slices.Contains(req.Rules, RulePins) {
		return repair
	}
	keeps := keepsOf(req)
	var findings []ste.Finding
	for _, finding := range repair.Findings {
		if finding.ID != pins.ID {
			findings = append(findings, finding)
		}
	}
	for _, finding := range pins.CheckPath(req.Path, req.Content) {
		if keeps(finding.ID) {
			findings = append(findings, finding)
		}
	}
	repair.Findings = findings
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
		Families: []string{string(RuleWrap), string(RuleSTE), string(RuleEnglish)},
		Rules:    append(append([]string{IDHardWrap}, slices.Sorted(ste.AllIDs.All())...), english.AllIDs...),
		Files:    []fixer.Kind{fixer.Document},
		Place:    30,
		Repair: func(f *fixer.File) {
			defer trace.Phase("fix/wrap-and-ste")()
			stePass := f.Wants(string(RuleSTE))
			englishPass := f.Wants(string(RuleEnglish)) && f.Keeps(english.IDCommaNever)
			word := func(text string) string {
				if englishPass {
					text = english.FixCommaNever(text)
				}
				if stePass {
					text = ste.FixSelected(text, f.Keeps)
				}
				return text
			}
			if _, safe := Format(f.Text()); safe {
				f.Apply(markdown.FormatEdits(f.Text(), word))
			}
		},
	})
}

// The layout pass runs last, on a Go file another fixer changed. It answers to
// the selection of the fixers before it.
func init() {
	var families, rules []string
	for _, fx := range fixer.For(fixer.Source) {
		families = append(families, fx.Categories()...)
		rules = append(rules, fx.IDs()...)
	}
	slices.Sort(families)
	slices.Sort(rules)
	fixer.Register(fixer.Spec{
		Label:    goformat.Name,
		Families: slices.Compact(families),
		Rules:    slices.Compact(rules),
		Files:    []fixer.Kind{fixer.Source},
		Place:    2000,
		Repair: func(f *fixer.File) {
			if f.Changed() && filepath.Ext(f.Path) == ".go" {
				defer trace.Phase("fix/gofmt")()
				f.ApplyThrough(goformat.Gate, goformat.Edits(f.Text()))
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
// Everything else is the caller's. A run that names a rule on the command line
// has to reach the repair, or the selection is silently ignored.
func FixFileWith(path string, req Request) (Repair, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Repair{}, err
	}
	req.Content, req.Path = string(content), path
	repair := Fix(req)
	if len(repair.Unmet) > 0 {
		return repair, &UnmetError{Path: path, Unmet: repair.Unmet}
	}
	if !repair.Changed {
		return repair, nil
	}
	return repair, commentfix.WriteFile(path, repair.Text)
}
