// Package slopfix is the library behind the binary. It reports what the org's
// prose rules reject, and rewrites what a rewrite can repair.
//
// The binary is a thin wrapper. A hook, a CI job and an editor integration
// all get identical answers instead of separate implementations that drift.
package slopfix

import (
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/pins"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/trace"
	"github.com/wow-look-at-my/slopfix/treecomments"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// IDHardWrap names the wrap rule: the document's shape is this package's to judge.
const IDHardWrap = "wrap/hard-wrap"

// Check reports every finding in a document that fails a check, in source order.
func Check(content string) []ste.Finding {
	var out []ste.Finding
	for _, block := range markdown.Split(content) {
		if block.Kind != markdown.Prose {
			continue
		}
		out = append(out, ste.Check(block.Text(), block.Start)...)
		out = append(out, english.CheckCommaNever(block.Text(), block.Start)...)
		for i := 1; i < len(block.Lines); i++ {
			out = append(out, ste.Finding{
				Line:   block.Start + i,
				ID:     IDHardWrap,
				Rule:   "a paragraph is one line",
				Detail: "this line continues the paragraph on line " + strconv.Itoa(block.Start),
				Fix:    "Join it back up and let the reader's window wrap it. `slopfix fix` does this.",
			})
		}
	}
	return append(out, longBlocks(content)...)
}

// Warnings reports every warning in a document. A repair never answers one, so
// Fix never reports one.
func Warnings(content string) []ste.Finding {
	var out []ste.Finding
	for _, block := range markdown.Split(content) {
		if block.Kind == markdown.Prose {
			out = append(out, ste.Warn(block.Text(), block.Start, block.Marker != "")...)
		}
	}
	return out
}

// Format joins every prose block to a single line, and reports whether the
// rewrite is safe. A result whose words differ from the source is a bug.
func Format(content string) (string, bool) {
	formatted := markdown.Format(content)
	return formatted, markdown.WordsOnly(content, formatted)
}

// CheckFile reads a file and reports its findings.
//
// A workflow and an action manifest are judged by the workflow rules, and every
// other file by the prose rules.
func CheckFile(path string) ([]ste.Finding, error) {
	read := trace.Phase("io/read")
	content, err := os.ReadFile(path)
	read()
	if err != nil {
		return nil, err
	}
	return CheckContent(path, string(content)), nil
}

// CheckContent reports the findings in text headed for path. The PATH decides:
// a Go file's lines are not paragraphs, so the prose rules skip it.
func CheckContent(path, content string) []ste.Finding {
	defer trace.Phase("check/file")()
	if exempt(path, content) {
		return nil
	}
	// A URL is text in every kind of file.
	return append(kindFindings(path, content), pins.CheckPath(path, content)...)
}

// kindFindings are the rules the file's kind selects.
func kindFindings(path, content string) []ste.Finding {
	if isWorkflow(path, content) {
		return append(workflow.Check(content), sentenceFindings(path, content)...)
	}
	if isDocument(path) {
		return append(Check(content), Warnings(content)...)
	}
	// A source file's lines are not paragraphs, so the prose rules stop here.
	return commentFindings(path, content)
}

// commentFindings are the source rules: what the comments in a source file
// break.
func commentFindings(path, content string) []ste.Finding {
	var out []ste.Finding
	for _, hit := range commentfix.Check(path, content) {
		out = append(out, ste.Finding{
			Line:   hit.Line,
			ID:     commentfix.ID,
			Rule:   "a number in a comment is a count of what exists today",
			Detail: hit.Number,
			Fix:    "Say it in words, or let the reader count. `slopfix fix` does this.",
		})
	}
	for _, hit := range commentfix.CheckLength(path, content) {
		fix := "Cut the comment back inside the code it documents. Drop the trailing paragraph first."
		if !hit.Repairable {
			fix = commentfix.FixLengthByHand
		}
		out = append(out, ste.Finding{
			Line:   hit.Line,
			ID:     hit.ID,
			Rule:   hit.Tell,
			Detail: hit.Sentence,
			Fix:    fix,
		})
	}
	for _, hit := range commentfix.CheckTails(path, content) {
		fix := "Finish the sentence, or let the repair close it. `slopfix fix` does this."
		if !hit.Repairable {
			fix = "Rewrite it by hand: finish the sentence. No cut leaves a whole sentence."
		}
		out = append(out, ste.Finding{
			Line:   hit.Line,
			ID:     hit.ID,
			Rule:   hit.Tell,
			Detail: hit.Sentence,
			Fix:    fix,
		})
	}
	return append(out, sentenceFindings(path, content)...)
}

// sentenceFindings are the sentences of a file's comments over the STE word
// cap. It is its own check, apart from the comment's weight against its code.
func sentenceFindings(path, content string) []ste.Finding {
	var out []ste.Finding
	for _, hit := range commentfix.CheckSentences(path, content) {
		out = append(out, ste.Finding{
			Line:    hit.Line,
			EndLine: hit.EndLine,
			ID:      ste.IDSentenceCap,
			Rule:    hit.Tell,
			Detail:  hit.Sentence,
			Fix:     hit.Fix,
		})
	}
	return out
}

// documentExtensions are the files whose lines are prose.
var documentExtensions = []string{".md", ".markdown", ".mdown", ".txt"}

// isDocument reports whether the prose rules own this file. An empty path is a
// document, because a caller holding text and naming no file is asking about
// prose rather than about a tree.
func isDocument(path string) bool {
	if path == "" {
		return true
	}
	if tombstones.InTestdata(path) || treecomments.HashComments(path) {
		return false
	}
	lower := strings.ToLower(path)
	for _, ext := range documentExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// AllIDs names every rule CheckContent reports, so a caller can reject a typo
// before it selects nothing and reads as a clean file.
func AllIDs() set.Set[string] {
	ids := workflow.AllIDs.Union(ste.AllIDs).Union(pins.AllIDs)
	ids.AddRange(IDHardWrap, IDLongBlock, commentfix.IDLength, commentfix.ID, commentfix.IDTail)
	ids.AddRange(english.AllIDs...)
	return ids
}

// Listed renders a set of rule IDs for a person. A set has no order of its
// own, so the reader gets an alphabetical listing.
func Listed(ids set.Set[string]) string {
	return strings.Join(slices.Sorted(ids.All()), ", ")
}

// isWorkflow reports whether the workflow rules own this file.
func isWorkflow(path, content string) bool {
	return workflow.Judges(path) || (isYAML(path) && workflow.Sniff(content))
}

func isYAML(path string) bool {
	return strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")
}
