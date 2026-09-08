// goblocks.go measures a Go file with Go's own parser.
//
// The line walk in blocks.go guesses where a declaration ends. That is
// tolerable for a report and not tolerable for a repair: this rule deletes
// comment text, and a span that is wrong by a line deletes the wrong prose.
// go/ast is in the standard library, needs no cgo, and answers exactly which
// node a doc comment is attached to, so the language this org writes most gets
// the exact answer rather than an approximation.
//
// A file that does not parse yields nothing. Half a syntax tree is a worse
// input than none, and a file mid-edit is the common case for a hook.
package commentlength

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// goBlocks pairs each doc comment in a Go file with the node it documents.
//
// ok is false when the file does not parse, and the caller then reports
// nothing rather than falling back to a guess.
func goBlocks(src string) (out []block, ok bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "src.go", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	lines := splitLines(src)

	// documented takes the parser's own attachment, which the line walk guessed at.
	for group, node := range documented(fset, file) {
		// The package doc is never measured. It introduces the file rather than
		// a declaration, so there is nothing of a comparable size to weigh it
		// against, and the check this rule replaces skips it for that reason.
		if group == file.Doc {
			continue
		}
		start := fset.Position(group.Pos()).Line - 1
		end := fset.Position(group.End()).Line
		if start < 0 || end > len(lines) || start >= end {
			continue
		}
		// A trailing comment shares its line with code. Measuring that line
		// counts the code as comment text, and cutting it deletes the code, so
		// this rule leaves one alone where commentspan measures it.
		if !startsComment(strings.TrimSpace(lines[start])) {
			continue
		}
		b := block{start: start, end: end, text: lines[start:end], exact: true}
		b.codeLines, b.codeChars = spanOf(fset, lines, node)
		out = append(out, b)
	}
	return out, true
}

// documented pairs each comment group with the node it belongs to.
//
// It asks ast.NewCommentMap, which is what commentspan asks. A hand-rolled walk
// over the declaration kinds saw a DOC comment and nothing else, so a comment
// inside a function body -- the commonest essay in this codebase -- was
// measured by the gate and reported by nothing here.
func documented(fset *token.FileSet, file *ast.File) map[*ast.CommentGroup]ast.Node {
	out := map[*ast.CommentGroup]ast.Node{}
	for node, groups := range ast.NewCommentMap(fset, file, file.Comments) {
		for _, g := range groups {
			out[g] = node
		}
	}
	return out
}

// spanOf measures the node a comment documents: how many source lines it
// occupies, and how many characters of code those lines hold.
//
// The span is followed however far the node runs, which is what commentspan
// does. A cap here reported a long function's proportionate comment as an
// essay, because the code it was weighed against stopped short.
func spanOf(fset *token.FileSet, lines []string, node ast.Node) (int, int) {
	from := fset.Position(node.Pos()).Line - 1
	to := fset.Position(node.End()).Line
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
