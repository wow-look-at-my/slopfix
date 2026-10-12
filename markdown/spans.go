package markdown

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/codespan"
)

// inlineNodes answers every code span of a document in source order, and how
// many stray backticks it holds.
func inlineNodes(content string) (nodes []string, stray int) {
	// A fence is a verbatim block, and its backticks are no code span; only prose holds inline nodes.
	for _, b := range Split(content) {
		if b.Kind != Prose {
			continue
		}
		text := strings.Join(b.Lines, "\n")
		for _, s := range codespan.Find(text) {
			nodes = append(nodes, text[s[0]:s[1]])
		}
		stray += len(codespan.Stray(text))
	}
	return nodes, stray
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

// KeepsNodes is the gate for a repair that moves markdown between files, as a
// split into docs/. Otherwise, a merge of CLAUDE.md into AGENTS.md does.
// Every code span and link the files held before stands, byte for byte. That
// span is in some file after, and no file holds more stray backticks than all
// the files did.
func KeepsNodes(before, after []string) error {
	held := set.New[string]()
	stray := 0
	for _, text := range after {
		spans, n := inlineNodes(text)
		held.AddRange(spans...)
		held.AddRange(links(text)...)
		stray += n
	}
	wasStray := 0
	for _, text := range before {
		spans, n := inlineNodes(text)
		wasStray += n
		for _, node := range append(spans, links(text)...) {
			if !held.Contains(node) {
				return fmt.Errorf("the repair lost %s", node)
			}
		}
	}
	if stray > wasStray {
		return fmt.Errorf("the repair left a stray backtick")
	}
	return nil
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
