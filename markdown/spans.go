package markdown

import (
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/codespan"
)

// inlineNodes answers every code span of a document in source order, and how
// many stray backticks it holds.
func inlineNodes(content string) (nodes []string, stray int) {
	for _, s := range codespan.Find(content) {
		nodes = append(nodes, content[s[0]:s[1]])
	}
	return nodes, len(codespan.Stray(content))
}

// links answers every link of a document in source order, as its label and
// its destination.
func links(content string) []string {
	src := []byte(blankFrontMatter(content, strings.Split(content, "\n")))
	var out []string
	_ = ast.Walk(parser.Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Link:
			var label strings.Builder
			_ = ast.Walk(v, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
				if t, ok := c.(*ast.Text); ok && entering {
					label.Write(t.Segment.Value(src))
				}
				return ast.WalkContinue, nil
			})
			out = append(out, "["+label.String()+"]("+string(v.Destination)+")")
		case *ast.AutoLink:
			out = append(out, "<"+string(v.URL(src))+">")
		}
		return ast.WalkContinue, nil
	})
	return out
}

// subsequence reports whether every item of part stands in whole, in order.
func subsequence(part, whole []string) bool {
	i := 0
	for _, p := range part {
		for i < len(whole) && whole[i] != p {
			i++
		}
		if i == len(whole) {
			return false
		}
		i++
	}
	return true
}

// spansKept reports whether a write kept the inline nodes it must not touch.
// Each code span after the write is one the text held, in the order it held
// them. A span named again when a repair restates a noun counts once. No new
// stray backtick appears. Every link the text held survives byte for byte
// and in order. A repair may add a link, as counts/section-number does.
func spansKept(before, after string) bool {
	was, wasStray := inlineNodes(before)
	now, nowStray := inlineNodes(after)
	if nowStray > wasStray {
		return false
	}
	seen := set.New[string]()
	var first []string
	for _, n := range now {
		if !seen.Contains(n) {
			seen.Add(n)
			first = append(first, n)
		}
	}
	if !subsequence(first, was) {
		return false
	}
	return subsequence(links(before), links(after))
}
