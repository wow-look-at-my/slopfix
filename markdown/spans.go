package markdown

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/wow-look-at-my/slopfix/codespan"
)

// inlineNodes answers every code span and link destination of a document in
// source order, and how many stray backticks it holds. The code spans are
// the ones every repair masks, so a stray backtick pairs with nothing here
// either.
func inlineNodes(content string) (nodes []string, stray int) {
	for _, s := range codespan.Find(content) {
		nodes = append(nodes, content[s[0]:s[1]])
	}
	lines := strings.Split(content, "\n")
	src := []byte(blankFrontMatter(content, lines))
	_ = ast.Walk(parser.Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Link:
			nodes = append(nodes, "link:"+string(v.Destination))
		case *ast.AutoLink:
			nodes = append(nodes, "autolink:"+string(v.URL(src)))
		}
		return ast.WalkContinue, nil
	})
	return nodes, len(codespan.Stray(content))
}

// spansKept reports whether every code span and link of after stands in
// before, byte for byte and in the same order. A repair may cut a span with
// the sentence that holds it. It never writes one, splits one, or leaves a
// stray backtick before did not hold.
func spansKept(before, after string) bool {
	was, wasStray := inlineNodes(before)
	now, nowStray := inlineNodes(after)
	if nowStray > wasStray {
		return false
	}
	i := 0
	for _, n := range now {
		for i < len(was) && was[i] != n {
			i++
		}
		if i == len(was) {
			return false
		}
		i++
	}
	return true
}
