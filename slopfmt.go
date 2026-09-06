// Package slopfmt is the library behind the binary. It reports what the org's
// prose rules reject, rewrites what a rewrite can repair, and purges the
// markdown files a repository must not keep.
//
// The binary is a thin wrapper, so a hook, a CI job and an editor integration
// all get identical answers instead of separate implementations that drift.
package slopfmt

import (
	"os"
	"strings"

	"github.com/wow-look-at-my/slopfmt/markdown"
	"github.com/wow-look-at-my/slopfmt/ste"
	"github.com/wow-look-at-my/slopfmt/workflow"
)

// IDHardWrap names the wrap rule. It lives here rather than in ste, because the
// document's shape is this package's to judge.
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
				Fix:  "Join it back up and let the reader's window wrap it. `slopfmt fmt` does this.",
			})
		}
		out = append(out, ste.Check(block.Text(), block.Start)...)
	}
	return out
}

// Format returns the document with every prose block joined to a single line.
//
// It reports whether the rewrite is safe to write back. Joining must only move
// newlines, so a result whose words differ from the source is a bug in the
// splitter, and the caller must keep the original.
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
	if isWorkflow(path, string(content)) {
		return workflow.Check(string(content)), nil
	}
	return Check(string(content)), nil
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
// joining a two-line concurrency: block makes GitHub reject the whole file.
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
