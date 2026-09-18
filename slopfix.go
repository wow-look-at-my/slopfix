// Package slopfix is the library behind the binary. It reports what the org's
// prose rules reject, rewrites what a rewrite can repair, and purges the
// markdown files a repository must not keep.
//
// The binary is a thin wrapper, so a hook, a CI job and an editor integration
// all get identical answers instead of separate implementations that drift.
package slopfix

import (
	"os"
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentlength"
	"github.com/wow-look-at-my/slopfix/commentnumbers"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// IDHardWrap names the wrap rule: the document's shape is this package's to judge.
const IDHardWrap = "wrap/hard-wrap"

// Check reports every finding in a document, in source order.
func Check(content string) []ste.Finding {
	var out []ste.Finding
	for _, block := range markdown.Split(content) {
		if block.Kind != markdown.Prose {
			continue
		}
		if len(block.Lines) > 1 {
			out = append(out, ste.Finding{
				Line: block.Start,
				ID:   IDHardWrap,
				Rule: "a paragraph is one line",
				Fix:  "Join it back up and let the reader's window wrap it. `slopfix fmt` does this.",
			})
		}
		out = append(out, ste.Check(block.Text(), block.Start)...)
	}
	return out
}

// Format joins every prose block to a single line, and reports whether the
// rewrite is safe: a result whose words differ from the source is a bug.
func Format(content string) (string, bool) {
	formatted := markdown.Format(content)
	return formatted, markdown.WordsOnly(content, formatted)
}

// CheckFile reads a file and reports its findings.
//
// A workflow and an action manifest are judged by the workflow rules, and every
// other file by the prose rules.
func CheckFile(path string) ([]ste.Finding, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return CheckContent(path, string(content)), nil
}

// CheckContent reports the findings in text headed for path. The PATH decides:
// a Go file's lines are not paragraphs, so the prose rules skip it.
func CheckContent(path, content string) []ste.Finding {
	if isWorkflow(path, content) {
		return workflow.Check(content)
	}
	if isDocument(path) {
		return Check(content)
	}
	// A source file's lines are not paragraphs, so the prose rules stop here.
	return commentFindings(path, content)
}

// commentFindings are the source rules: what the comments in a source file
// break, reported on the line they sit on.
func commentFindings(path, content string) []ste.Finding {
	var out []ste.Finding
	for _, hit := range commentnumbers.Check(path, content) {
		out = append(out, ste.Finding{
			Line:   hit.Line,
			ID:     commentnumbers.ID,
			Rule:   "a number in a comment is a count of what exists today",
			Detail: hit.Number,
			Fix:    "Say it in words, or let the reader count. `slopfix fix` does this.",
		})
	}
	for _, hit := range commentlength.Check(path, content) {
		fix := "Cut the comment back inside the code it documents. Drop the trailing paragraph first."
		if !hit.Repairable {
			fix = "Shorten the opening sentence, or say less."
		}
		out = append(out, ste.Finding{
			Line:   hit.Line,
			ID:     hit.ID,
			Rule:   hit.Tell,
			Detail: hit.Sentence,
			Fix:    fix,
		})
	}
	return out
}

// documentExtensions are the files whose lines really are prose.
var documentExtensions = []string{".md", ".markdown", ".mdown", ".txt"}

// isDocument reports whether the prose rules own this file. An empty path is a
// document, because a caller holding text and naming no file is asking about
// prose rather than about a tree.
func isDocument(path string) bool {
	if path == "" {
		return true
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
	ids := workflow.AllIDs.Union(ste.AllIDs)
	ids.AddRange(IDHardWrap, commentlength.ID, commentnumbers.ID)
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

// FormatFile rewrites a file in place and reports whether it changed. It never
// writes a rewrite that lost a word.
//
// A workflow is refused rather than joined. A newline is syntax there, and
// joining a wrapped concurrency: block makes GitHub reject the whole file.
func FormatFile(path string) (changed bool, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if isWorkflow(path, string(content)) {
		return false, errNotProse{path}
	}
	formatted, safe := Format(string(content))
	if !safe {
		return false, errLossy{path}
	}
	if formatted == string(content) {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(formatted), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

// errLossy is the refusal to write a rewrite that changed the words.
type errLossy struct{ path string }

func (e errLossy) Error() string {
	return e.path + ": refusing to write -- the rewrite changed the words, not only the line breaks"
}

// errNotProse is the refusal to reflow a file whose newlines are syntax.
type errNotProse struct{ path string }

func (e errNotProse) Error() string {
	return e.path + ": refusing to format -- a workflow's newlines are syntax, and joining them breaks the file"
}
