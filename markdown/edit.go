// edit.go is the door a repair writes a document through.
//
// An edit must sit inside a single prose block, as the CommonMark parser found
// it. After the splice the document is parsed again, and every verbatim block
// must come back as it was, in the same order. Text that opens a fence, starts
// a heading or ends a list changes that answer, so the edit that wrote it
// never lands. An edit may add a blank line that divides a paragraph. The
// containers must then hold the same paragraphs in the same places.
package markdown

import (
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/wow-look-at-my/slopfix/edit"
)

// Apply writes the edits into content through the gate. An edit outside the
// scope is dropped without a finding. An edit that leaves its prose block, or
// that changes a verbatim block once written, is refused.
func Apply(content string, edits []edit.Edit, scope edit.Scope) edit.Result {
	if len(edits) == 0 {
		return edit.Unchanged(content, scope)
	}
	spans := proseSpans(content)
	want, wantShape := verbatim(content), shape(content)
	return edit.Gate(content, edits, scope,
		func(e edit.Edit) string {
			// The spans run in source order, so the only candidate is the last one that opens at or before the edit.
			i := sort.Search(len(spans), func(i int) bool { return spans[i][0] > e.Start }) - 1
			if i >= 0 && e.End <= spans[i][1] {
				return ""
			}
			return "it reaches past its prose block"
		},
		func(text string) bool { return sameLines(verbatim(text), want) && shape(text) == wantShape })
}

// shape renders the container tree of a document. A run of paragraphs in one
// container renders as a single P, so a divided paragraph keeps the shape. A
// paragraph that leaves its list item changes it.
func shape(content string) string {
	lines := strings.Split(content, "\n")
	src := []byte(blankFrontMatter(content, lines))
	var b strings.Builder
	_ = ast.Walk(parser.Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		paragraph := n.Kind() == ast.KindParagraph || n.Kind() == ast.KindTextBlock
		if !entering {
			if !paragraph {
				b.WriteByte(')')
			}
			return ast.WalkContinue, nil
		}
		if paragraph {
			if prev := n.PreviousSibling(); prev == nil || (prev.Kind() != ast.KindParagraph && prev.Kind() != ast.KindTextBlock) {
				b.WriteString("P")
			}
			return ast.WalkSkipChildren, nil
		}
		if n.Type() == ast.TypeInline {
			return ast.WalkSkipChildren, nil
		}
		b.WriteString(n.Kind().String() + "(")
		return ast.WalkContinue, nil
	})
	return b.String()
}

// BlockEdit is an edit that replaces a whole prose block with lines.
func BlockEdit(content string, b Block, lines []string) edit.Edit {
	return edit.Lines(content, b.Start-1, b.Start-2+len(b.Lines), 0, lines)
}

// proseSpans answers the bytes each prose block covers, line ends inside it
// included and the last one left out.
func proseSpans(content string) [][2]int {
	var out [][2]int
	for _, b := range Split(content) {
		if b.Kind != Prose {
			continue
		}
		e := BlockEdit(content, b, []string{""})
		out = append(out, [2]int{e.Start, e.End})
	}
	return out
}

// verbatim answers every line the parser keeps as written, in order. A blank
// line is not in the list, because shape holds what a blank line divides.
func verbatim(content string) []string {
	var out []string
	for _, b := range Split(content) {
		if b.Kind == Verbatim && strings.TrimSpace(strings.Join(b.Lines, "")) != "" {
			out = append(out, strings.Join(b.Lines, "\n"))
		}
	}
	return out
}

func sameLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
