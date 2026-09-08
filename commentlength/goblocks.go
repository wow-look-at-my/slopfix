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

	// documented pairs a comment group with the node it is attached to. The
	// parser has already decided that attachment, which is the judgement the
	// line walk was making for itself.
	for group, node := range documented(file) {
		start := fset.Position(group.Pos()).Line - 1
		end := fset.Position(group.End()).Line
		if start < 0 || end > len(lines) || start >= end {
			continue
		}
		b := block{start: start, end: end, text: lines[start:end], exact: true}
		b.codeLines, b.codeChars = spanOf(fset, lines, node)
		out = append(out, b)
	}
	return out, true
}

// documented walks the file for every node carrying a doc comment.
// Only a doc comment is measured: a free-floating comment documents nothing.
func documented(file *ast.File) map[*ast.CommentGroup]ast.Node {
	out := map[*ast.CommentGroup]ast.Node{}
	add := func(doc *ast.CommentGroup, node ast.Node) {
		if doc != nil && node != nil {
			out[doc] = node
		}
	}
	add(file.Doc, file)

	ast.Inspect(file, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.GenDecl:
			add(d.Doc, d)
		case *ast.FuncDecl:
			add(d.Doc, d)
		case *ast.TypeSpec:
			add(d.Doc, d)
		case *ast.ValueSpec:
			add(d.Doc, d)
		case *ast.Field:
			add(d.Doc, d)
		}
		return true
	})
	return out
}

// spanOf measures the node a comment documents: how many source lines it
// occupies, and how many characters of code those lines hold.
//
// A package clause is the exception. The file node spans the whole file, and
// weighing a package comment against every line below it means no package
// comment can ever be too long. It is measured against its own clause.
func spanOf(fset *token.FileSet, lines []string, node ast.Node) (int, int) {
	from := fset.Position(node.Pos()).Line - 1
	to := fset.Position(node.End()).Line
	if file, isFile := node.(*ast.File); isFile {
		from = fset.Position(file.Name.Pos()).Line - 1
		to = from + 1
	}
	if from < 0 {
		from = 0
	}
	if to > len(lines) {
		to = len(lines)
	}
	if to > from+maxCodeLines {
		to = from + maxCodeLines
	}

	count, chars := 0, 0
	for _, line := range lines[from:to] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || startsComment(trimmed) {
			continue
		}
		count++
		chars += len(trimmed)
	}
	return count, chars
}
