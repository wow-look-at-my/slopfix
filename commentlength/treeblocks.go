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
	"fmt"
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
			run, stop := commentRun(node, i, count)
			next := afterComments(node, stop, count)
			header := root && i == 0 && documentsThePackage(node, next, count)
			i = stop - 1
			// A comment above the package declaration introduces the package
			// rather than a construct, so there is nothing of a comparable size
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
	b := block{start: start, end: end, text: lines[start:end], exact: true}
	// A run with nothing after it documents nothing. The package doc is the
	// exemption, and it is taken before this, so what is left is prose trailing
	// off the end of a file with no construct to weigh it against.
	if next >= count {
		b.documents = "nothing"
		return b, true
	}
	documented := firstStatement(parent.NamedChild(next))
	b.codeLines, b.codeChars = nodeSpan(documented, lines)
	b.documents = fmt.Sprintf("%s@%d-%d/in:%s", documented.Type(),
		documented.StartPoint().Row+1, documented.EndPoint().Row+1, parent.Type())
	return b, true
}

// firstStatement descends through a bare sequence to the construct a comment
// actually documents.
//
// A grammar can group everything left in a block into a single node. Weighing a
// comment against that node measures the rest of the block, so a note over a
// lone statement reads as proportionate to lines it does not describe.
//
// A bare sequence is recognised without naming a language, by two properties it
// has and a construct does not. It opens on its first child, where a construct
// opens on a keyword or a brace of its own. And its children each begin on a
// line of their own, where the parts of one statement share lines. The descent
// stops unless it saves lines, so a statement spread over several stays whole.
func firstStatement(node ts.Node) ts.Node {
	for !node.IsNull() && isSequence(node) {
		node = node.NamedChild(0)
	}
	return node
}

func isSequence(node ts.Node) bool {
	count := node.NamedChildCount()
	if count == 0 || node.StartByte() != node.NamedChild(0).StartByte() {
		return false
	}
	if node.NamedChild(0).EndPoint().Row >= node.EndPoint().Row {
		return false
	}
	seen := node.NamedChild(0).StartPoint().Row
	for i := uint32(1); i < count; i++ {
		row := node.NamedChild(i).StartPoint().Row
		if row <= seen {
			return false
		}
		seen = row
	}
	return true
}

// afterComments advances past a comment run the pairing must not measure.
//
// A blank line ends a run, so the node after it can be more prose. Measuring a
// comment against a comment gives no code at all, and the block is then dropped
// with nothing said about it.
func afterComments(node ts.Node, next, count uint32) uint32 {
	for next < count && isComment(node.NamedChild(next)) {
		next++
	}
	return next
}

// documentsThePackage reports whether the construct after a file's opening
// comment run declares the package the file belongs to.
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
