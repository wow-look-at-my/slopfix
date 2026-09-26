// Package tombstones finds a comment that describes a state the code is no
// longer in, or that argues for the diff instead of telling the next editor
// what breaks.
//
// Only prose is judged. In source that is the comments, never the code, so a
// string literal holding the word "previously" is not a tombstone. In a
// document it is the prose, with fences and frontmatter skipped.
//
// The input may be a fragment rather than a whole file, so a scanner starting
// mid-string can read code as a comment. That costs a false positive at worst.
package tombstones

import (
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/slopfix/code"
	"github.com/wow-look-at-my/slopfix/markdown"
)

// Block is a comment run or a paragraph. LineNos and Pure place each line and
// judge it, and are nil for a paragraph.
type Block struct {
	Text    string
	Lines   int
	LineNos []int
	Pure    []bool
}

// AddedBlocks returns the prose that added contributes to path. It returns nil
// for a path no grammar parses.
func AddedBlocks(path, added string) []Block {
	if IsDocument(path) {
		return paragraphs(added)
	}
	runs, ok := code.Runs(path, added)
	if !ok {
		return nil
	}
	lines := strings.Split(added, "\n")
	out := make([]Block, 0, len(runs))
	for _, run := range runs {
		lineNos := make([]int, 0, run.End-run.Start)
		for no := run.Start; no < run.End; no++ {
			lineNos = append(lineNos, no)
		}
		out = append(out, Block{
			Text:    strings.Join(lines[run.Start:run.End], "\n"),
			Lines:   run.End - run.Start,
			LineNos: lineNos,
			Pure:    run.Pure,
		})
	}
	return out
}

// IsDocument reports whether path names prose rather than source.
func IsDocument(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".rst", ".txt", ".adoc":
		return true
	}
	return false
}

// paragraphs answers the prose blocks the CommonMark parser finds. What is not
// the document's own voice stays out: code, headings, quotations, HTML and
// frontmatter.
func paragraphs(doc string) []Block {
	var out []Block
	for _, b := range markdown.Split(doc) {
		if b.Kind != markdown.Prose {
			continue
		}
		cur := make([]string, 0, len(b.Lines))
		nos := make([]int, 0, len(b.Lines))
		for n, line := range b.Lines {
			cur = append(cur, blankInlineCode(line))
			nos = append(nos, b.Start-1+n)
		}
		out = append(out, Block{Text: strings.Join(cur, "\n"), Lines: len(cur), LineNos: nos})
	}
	return out
}

// blankInlineCode replaces each backtick span with spaces, keeping every byte
// offset. A phrase inside verbatim machinery is a literal, not a claim.
func blankInlineCode(line string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(line); i++ {
		if line[i] == '`' {
			in = !in
			b.WriteByte(' ')
			continue
		}
		if in {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(line[i])
	}
	return b.String()
}
