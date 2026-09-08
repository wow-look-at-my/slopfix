// treeblocks.go pairs a comment with the construct it documents, from a real
// syntax tree.
//
// It replaced a line walk that guessed where a declaration ended, and a Go-only
// path built on go/ast beside it. The walk could not be repaired from, because
// a span wrong by a line deletes the wrong prose, so the fix ran for Go alone.
// A tree gives every language an exact span, so the fix runs everywhere.
//
// Nothing here names a language. A comment is a node whose type carries
// "comment", which is how every grammar spells it, and the documented construct
// is the next named sibling. Both facts come from the tree.
package commentlength

import (
	"path/filepath"
	"strings"

	ts "github.com/wow-look-at-my/go-tree-sitter"
	"github.com/wow-look-at-my/slopfix/grammars/bash"
	"github.com/wow-look-at-my/slopfix/grammars/clang"
	"github.com/wow-look-at-my/slopfix/grammars/cpp"
	"github.com/wow-look-at-my/slopfix/grammars/golang"
	"github.com/wow-look-at-my/slopfix/grammars/rust"
)

// grammars maps a file extension to the grammar that parses it.
var grammars = map[string]func() *ts.Language{
	".go":   golang.Language,
	".c":    clang.Language,
	".h":    clang.Language,
	".cc":   cpp.Language,
	".cpp":  cpp.Language,
	".cxx":  cpp.Language,
	".hpp":  cpp.Language,
	".hh":   cpp.Language,
	".rs":   rust.Language,
	".sh":   bash.Language,
	".bash": bash.Language,
}

// languageFor answers the grammar for a filename, and nil when none parses it.
func languageFor(filename string) *ts.Language {
	if load, ok := grammars[strings.ToLower(filepath.Ext(filename))]; ok {
		return load()
	}
	return nil
}

// Parsed reports whether this rule parses a file rather than skipping it.
func Parsed(filename string) bool { return languageFor(filename) != nil }

// treeBlocks pairs each comment run in a file with the construct beneath it.
//
// ok is false when the file does not parse, and the caller then reports nothing
// rather than measuring against a broken tree. A file mid-edit is the common
// case for a hook, and half a tree is a worse input than none.
func treeBlocks(language *ts.Language, src string) (out []block, ok bool) {
	parser := ts.NewParser()
	if !parser.SetLanguage(language) {
		return nil, false
	}
	tree := parser.ParseString(nil, []byte(src))
	if tree == nil {
		return nil, false
	}
	root := tree.RootNode()
	if root.IsNull() || root.HasError() {
		return nil, false
	}

	lines := splitLines(src)
	collect(root, true, lines, &out)
	sortBlocks(out)
	return out, true
}

// collect walks a node's children, gathering each run of comments with the
// construct that follows it, then recurses. A comment inside a function body
// is found the same way as a comment above a declaration.
func collect(node ts.Node, root bool, lines []string, out *[]block) {
	count := node.NamedChildCount()
	for i := uint32(0); i < count; i++ {
		child := node.NamedChild(i)
		if isComment(child) {
			run, next := commentRun(node, i, count)
			header := root && i == 0 && documentsThePackage(node, next, count)
			i = next - 1
			// A comment above the package declaration introduces the package
			// rather than a construct, so there is nothing of a comparable size
			// to weigh it against.
			//
			// Being FIRST is not enough on its own. A shell script opens with a
			// comment that documents the assignment under it, and skipping every
			// file's opening run reported nothing for those files at all.
			if header {
				continue
			}
			if b, ok := blockFor(run, node, next, count, lines); ok {
				*out = append(*out, b)
			}
			continue
		}
		collect(child, false, lines, out)
	}
}

// commentRun gathers the comments starting at index i that sit on adjoining
// lines, and returns the index after the run.
func commentRun(node ts.Node, i, count uint32) ([]ts.Node, uint32) {
	run := []ts.Node{node.NamedChild(i)}
	j := i + 1
	for ; j < count; j++ {
		next := node.NamedChild(j)
		if !isComment(next) {
			break
		}
		last := run[len(run)-1]
		if next.StartPoint().Row > last.EndPoint().Row+1 {
			break
		}
		run = append(run, next)
	}
	return run, j
}

// blockFor measures a comment run against the construct it documents.
func blockFor(run []ts.Node, parent ts.Node, next, count uint32, lines []string) (block, bool) {
	start := int(run[0].StartPoint().Row)
	end := int(run[len(run)-1].EndPoint().Row) + 1
	if start < 0 || end > len(lines) || start >= end {
		return block{}, false
	}
	// A trailing comment shares its line with code. Measuring that line counts
	// the code as comment text, and cutting it deletes the code.
	if !startsComment(strings.TrimSpace(lines[start])) {
		return block{}, false
	}
	if next >= count {
		return block{}, false
	}
	b := block{start: start, end: end, text: lines[start:end], exact: true}
	b.codeLines, b.codeChars = nodeSpan(parent.NamedChild(next), lines)
	return b, true
}

// documentsThePackage reports whether the construct after a file's opening
// comment run declares the package the file belongs to.
//
// It reads the node type rather than the file extension, so it stays a fact
// about the tree. Go spells it package_clause; a grammar without the concept
// answers no, and its opening comment is measured like any other.
func documentsThePackage(node ts.Node, next, count uint32) bool {
	if next >= count {
		return false
	}
	return strings.Contains(node.NamedChild(next).Type(), "package")
}

// isComment reports a node every grammar spells as a comment.
func isComment(node ts.Node) bool {
	return !node.IsNull() && strings.Contains(node.Type(), "comment")
}

// nodeSpan measures a construct: the source lines it occupies, and the
// characters of code those lines hold.
//
// The span follows the node however far it runs. A cap here reported a long
// function's proportionate comment as an essay, because the code it was weighed
// against stopped short.
func nodeSpan(node ts.Node, lines []string) (int, int) {
	if node.IsNull() {
		return 0, 0
	}
	from := int(node.StartPoint().Row)
	to := int(node.EndPoint().Row) + 1
	if from < 0 {
		from = 0
	}
	if to > len(lines) {
		to = len(lines)
	}
	code := make([]string, 0, to-from)
	for _, line := range lines[from:to] {
		if trimmed := strings.TrimSpace(line); trimmed == "" || startsComment(trimmed) {
			continue
		}
		code = append(code, line)
	}
	// The same measure the comment gets, so both counts compare directly.
	return measure(code)
}

// sortBlocks puts the blocks in file order. Fix splices back to front, so an
// out-of-order answer rewrites with stale line numbers.
func sortBlocks(out []block) {
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].start < out[j-1].start; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
}
