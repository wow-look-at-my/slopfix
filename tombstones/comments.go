// Package tombstones finds a comment that describes a state the code is no
// longer in. It also finds a comment that argues for the diff instead of
// telling the next editor what breaks.
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
	"slices"
	"strings"
	"sync"

	"github.com/wow-look-at-my/slopfix/code"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/gitmod"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// Block is a comment run or a paragraph. LineNos and Pure place each line and
// judge it, and are nil for a paragraph.
type Block struct {
	Text    string
	Lines   int
	LineNos []int
	Pure    []bool
	// Prefix is the indentation and list marker that open a document paragraph. A rewrite writes it back, or the paragraph leaves its list.
	Prefix string
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
		// A line shared with code is a note on that code, and a directive is an instruction to a tool.
		pure := 0
		for idx, p := range run.Pure {
			if p && !treecomments.IsDirective(lines[run.Start+idx]) {
				pure++
			}
		}
		out = append(out, Block{
			Text:    strings.Join(lines[run.Start:run.End], "\n"),
			Lines:   pure,
			LineNos: lineNos,
			Pure:    run.Pure,
		})
	}
	return out
}

// IsDocument reports whether path names prose rather than source.
func IsDocument(path string) bool {
	// A CMakeLists.txt is code that ends in .txt.
	if InTestdata(path) || treecomments.HashComments(path) {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".rst", ".txt", ".adoc":
		return true
	}
	return false
}

// InTestdata reports whether path sits under a testdata directory.
func InTestdata(path string) bool { return under(path, "testdata") }

// Borrowed reports a path another author wrote: one under a vendor or node_modules directory.
func Borrowed(path string) bool { return under(path, "vendor", "node_modules") || vendoredAttr(path) }

// vendoredCache holds each path's answer, because a hook asks per write.
var vendoredCache sync.Map

// vendoredAttr asks git whether .gitattributes sets any commentfix.BorrowedAttributes on
// path. A path outside a work tree, or one git cannot answer for, is not borrowed.
func vendoredAttr(path string) bool {
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	if v, ok := vendoredCache.Load(abs); ok {
		return v.(bool)
	}
	args := append(append([]string{"check-attr", "-z"}, commentfix.BorrowedAttributes...), "--", filepath.Base(abs))
	out, err := gitmod.Command(filepath.Dir(abs), args...).Output()
	vendored := false
	if err == nil {
		// git answers a path, attribute, value triple per attribute.
		fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
		for i := 0; i+2 < len(fields); i += 3 {
			if commentfix.AttributeSet(fields[i+2]) {
				vendored = true
			}
		}
	}
	vendoredCache.Store(abs, vendored)
	return vendored
}

// under reports whether a directory element of path is one of dirs.
func under(path string, dirs ...string) bool {
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if slices.Contains(dirs, part) {
			return true
		}
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
		prefix := b.Indent
		if b.Marker != "" {
			prefix += b.Marker + " "
		}
		out = append(out, Block{Text: strings.Join(cur, "\n"), Lines: len(cur), LineNos: nos, Prefix: prefix})
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
