package slopfix

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// The reads a rule's own detect and autofix functions are written on. Each rule
// file names its own pair and registers itself, so no file holds a list of the
// rules. A rule file that is deleted takes its rule with it.
//
// A case is a materialized fixture: a repository under Root and a message in
// Text. Every read covers all of it, so no rule is blind to a part of a
// fixture another rule's case put there.

// fixtureFiles answers every regular file of the case's repository outside
// .git, by absolute path, in order.
func fixtureFiles(c RuleCase) []string {
	if c.Root == "" {
		return nil
	}
	var out []string
	err := filepath.WalkDir(c.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("rule case %q: %v", c.Name, err))
	}
	sort.Strings(out)
	return out
}

// readFixture answers a file of the case's repository.
func readFixture(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(content)
}

// eachText hands every text of the case to repair: each file of the
// repository, then the message. A file whose text changes is written back.
func eachText(c RuleCase, repair func(text string) string) RuleCase {
	for _, path := range fixtureFiles(c) {
		before := readFixture(path)
		if after := repair(before); after != before {
			if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
				panic(err)
			}
		}
	}
	if c.Text != "" {
		c.Text = repair(c.Text)
	}
	return c
}

// allTexts answers every text of the case: each file of the repository, then
// the message.
func allTexts(c RuleCase) []string {
	var out []string
	for _, path := range fixtureFiles(c) {
		out = append(out, readFixture(path))
	}
	if c.Text != "" {
		out = append(out, c.Text)
	}
	return out
}

// fixtureDigest names the state of a case, so a read of an unchanged fixture
// is answered once for every rule that asks.
func fixtureDigest(kind string, c RuleCase) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%d\x00%s\x00", kind, c.Root, len(c.Text), c.Text)
	for _, path := range fixtureFiles(c) {
		content := readFixture(path)
		fmt.Fprintf(h, "%s\x00%d\x00%s\x00", path, len(content), content)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// fixtureReads holds the findings of each fixture state a read has answered.
var fixtureReads = struct {
	sync.Mutex
	by map[string][]ste.Finding
}{by: map[string][]ste.Finding{}}

// fixtureReadCap bounds fixtureReads. The harness reads one fixture at a time.
const fixtureReadCap = 64

// remembered answers read for the case, from fixtureReads when this state was
// read before.
func remembered(kind string, c RuleCase, read func() []ste.Finding) []ste.Finding {
	key := fixtureDigest(kind, c)
	fixtureReads.Lock()
	got, ok := fixtureReads.by[key]
	fixtureReads.Unlock()
	if ok {
		return got
	}
	got = read()
	fixtureReads.Lock()
	defer fixtureReads.Unlock()
	if len(fixtureReads.by) >= fixtureReadCap {
		clear(fixtureReads.by)
	}
	fixtureReads.by[key] = got
	return got
}

// only answers the findings of one rule.
func only(findings []ste.Finding, id string) []ste.Finding {
	var out []ste.Finding
	for _, f := range findings {
		if f.ID == id {
			out = append(out, f)
		}
	}
	return out
}

// keptFinding writes a kept tombstone as the finding it reports.
func keptFinding(k tombstones.Hit) ste.Finding {
	return ste.Finding{Line: k.LineNo, ID: k.ID, Rule: k.Tell, Detail: k.Phrase, Fix: k.Fix}
}

// fileReads answers what check and a report find in every file of the case a
// file rule reads. And in the message read as a document with no path.
func fileReads(c RuleCase) []ste.Finding {
	return remembered("file", c, func() []ste.Finding {
		var out []ste.Finding
		read := func(path, text string) {
			out = append(out, CheckContent(path, text)...)
			rep := Report(Request{Content: text, Path: path, MaxCommentLines: tombstones.DefaultMaxCommentLines})
			out = append(out, rep.Findings...)
			for _, k := range rep.Kept {
				out = append(out, keptFinding(k))
			}
		}
		if c.Root != "" {
			for _, path := range commentfix.TreeFilesMatching(c.Root, Reads) {
				read(path, readFixture(path))
			}
		}
		if c.Text != "" {
			read("", c.Text)
		}
		return out
	})
}

// treeReads answers what a check of the whole repository finds.
func treeReads(c RuleCase) []ste.Finding {
	return remembered("tree", c, func() []ste.Finding {
		if c.Root == "" {
			return nil
		}
		run := CheckTreeWith(c.Root, Request{MaxCommentLines: tombstones.DefaultMaxCommentLines})
		var out []ste.Finding
		for _, f := range run.Findings {
			out = append(out, f.Finding)
		}
		for _, k := range run.Kept {
			out = append(out, keptFinding(k.Hit))
		}
		return out
	})
}

// caseFindings answers the findings one file rule reports anywhere in the case.
func caseFindings(c RuleCase, id string) []ste.Finding {
	return only(fileReads(c), id)
}

// treeFindings answers the findings one repository rule reports over the case.
func treeFindings(c RuleCase, id string) []ste.Finding {
	return only(treeReads(c), id)
}

// caseAutofix answers the case with one rule's repair applied to the whole
// repository, as `slopfix fix. --only ID` writes it, and to the message.
func caseAutofix(c RuleCase, id string) RuleCase {
	if c.Root != "" {
		FixTreeWith(c.Root, Request{IDs: []string{id}, MaxCommentLines: tombstones.DefaultMaxCommentLines})
	}
	if c.Text != "" {
		c.Text = Fix(Request{Content: c.Text, IDs: []string{id}, MaxCommentLines: tombstones.DefaultMaxCommentLines}).Text
	}
	return c
}

// messageFindings answers what one message rule reports in every text of the
// case: the message, and each file of the repository read as a message.
func messageFindings(c RuleCase, id string) []ste.Finding {
	runs := func(rule string) bool { return rule == id }
	var out []ste.Finding
	for _, text := range allTexts(c) {
		out = append(out, CheckMessage(text, runs)...)
	}
	return out
}

// messageAutofix cuts what one message rule reports out of every text of the
// case, as `slopfix fix --message --only ID` does.
func messageAutofix(c RuleCase, id string) RuleCase {
	runs := func(rule string) bool { return rule == id }
	return eachText(c, func(text string) string { return FixMessage(text, runs) })
}

// treeAutofix answers the case with one repository rule's repair applied.
func treeAutofix(c RuleCase, id string) RuleCase { return caseAutofix(c, id) }

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
		var out []ste.Finding
		for _, text := range allTexts(c) {
			for _, p := range patterns {
				if _, took := p.ApplyN(text); took > 0 {
					out = append(out, ste.Finding{
						Line:   1,
						ID:     id,
						Rule:   "a phrase about a state the code has left",
						Detail: p.Match,
						Fix:    "Cut the phrase. `slopfix fix` does this.",
					})
				}
			}
		}
		return out
	}
}

// repairPattern rewrites a wording rule's phrases out of every text of the case.
func repairPattern(patterns []english.Pattern) func(RuleCase) RuleCase {
	return func(c RuleCase) RuleCase {
		return eachText(c, func(text string) string {
			for _, p := range patterns {
				text = p.Apply(text)
			}
			return text
		})
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
