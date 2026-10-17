// edit.go is the door a repair writes source through.
//
// An edit names bytes the tree calls comment. The gate refuses any byte it
// covers that is neither comment nor the blank around it. After the splice
// the source is parsed again. Every node that is not a comment must come back
// with the same type and the same text, in the same place in the tree. Some
// text changes that tree: an early comment close, a newline that ends a line
// comment, or a marker that opens a new one. The edit that wrote such text
// never lands. Every directive line must also come back as it was.
package treecomments

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	ts "github.com/wow-look-at-my/go-tree-sitter"
	"github.com/wow-look-at-my/slopfix/code"
	"github.com/wow-look-at-my/slopfix/edit"
)

// Apply writes the edits into src through the gate. An edit outside the scope
// is dropped without a finding: the caller asked for no change there. An edit
// that reaches code, or that changes the code tree once written, is refused.
func Apply(filename, src string, edits []edit.Edit, scope edit.Scope) edit.Result {
	if len(edits) == 0 {
		return edit.Unchanged(src, scope)
	}
	refuseAll := func(reason string) edit.Result {
		out := edit.Unchanged(src, scope)
		for _, e := range edits {
			out.Refused = append(out.Refused, edit.Refused{Edit: e, Reason: reason})
		}
		return out
	}
	language := languageFor(filename)
	if language == nil {
		return refuseAll("no grammar reads " + filename)
	}
	root, ok := parseWith(language, src)
	if !ok {
		return refuseAll("the source does not parse")
	}
	// Gate drops an out-of-scope edit first, so a run that lands nothing reads none of these.
	spans := sync.OnceValue(func() []span { return commentSpans(filename, src) })
	want := sync.OnceValue(func() []shapeNode { return shape(root, src) })
	directives := sync.OnceValue(func() []string { return directiveLines(filename, src) })
	return edit.Gate(src, edits, scope,
		func(e edit.Edit) string { return reach(src, spans(), e) },
		func(text string) bool {
			return sameShape(language, text, want()) &&
				slices.Equal(directiveLines(filename, text), directives()) &&
				(!IsCMake(filename) || sameCMake(src, text))
		})
}

// span is where a comment node sits.
type span struct{ start, end int }

// commentSpans answers every comment a repair may write into. The interpreter
// line and a cgo preamble are left out, because a tool reads them as code.
func commentSpans(filename, src string) []span {
	var out []span
	for _, c := range Extract(filename, src) {
		out = append(out, span{start: c.Offset, end: c.Offset + len(c.Text)})
	}
	return out
}

// reach answers why an edit covers bytes that are not its own, or "".
func reach(src string, spans []span, e edit.Edit) string {
	touches := false
	for at := e.Start; at < e.End; at++ {
		if inside(spans, at) {
			touches = true
			continue
		}
		if !blank(src[at]) {
			return fmt.Sprintf("byte %d is code, not comment", at)
		}
	}
	// A pure insertion lands at a comment's edge or inside it.
	if e.Start == e.End {
		touches = inside(spans, e.Start) || (e.Start > 0 && inside(spans, e.Start-1))
	}
	if !touches {
		return "it touches no comment"
	}
	return ""
}

// inside reports whether the byte at is part of a comment.
func inside(spans []span, at int) bool {
	i := sort.Search(len(spans), func(i int) bool { return spans[i].end > at })
	return i < len(spans) && spans[i].start <= at
}

func blank(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

// shape renders every node that is not a comment: its depth, its type, and
// for a token its text. Whitespace is no node, so a comment edit that stays a
// comment leaves the shape as it was.
func shape(root ts.Node, src string) []shapeNode {
	var out []shapeNode
	walkShape(root, src, func(s shapeNode) bool {
		out = append(out, s)
		return true
	})
	return out
}

// shapeNode is one node of a shape.
type shapeNode struct {
	depth   int
	kind    string
	missing bool
	// text is a token's source, and hasText marks a token whose bytes lie in the source.
	text    string
	hasText bool
}

// walkShape hands visit each node of the shape in order, and stops at the
// earliest one visit refuses.
func walkShape(root ts.Node, src string, visit func(shapeNode) bool) bool {
	var walk func(n ts.Node, depth int) bool
	walk = func(n ts.Node, depth int) bool {
		kind := n.Type()
		if strings.Contains(kind, "comment") {
			return true
		}
		s := shapeNode{depth: depth, kind: kind, missing: n.IsMissing()}
		if n.ChildCount() == 0 {
			start, end := int(n.StartByte()), int(n.EndByte())
			if start >= 0 && end <= len(src) && start <= end {
				s.text, s.hasText = src[start:end], true
			}
			return visit(s)
		}
		if !visit(s) {
			return false
		}
		for _, child := range code.Children(n) {
			if !walk(child, depth+1) {
				return false
			}
		}
		return true
	}
	return walk(root, 0)
}

// sameShape reports whether text parses to the code tree want describes. It
// stops at the earliest node that differs.
func sameShape(language *ts.Language, text string, want []shapeNode) bool {
	root, ok := parseWith(language, text)
	if !ok {
		return false
	}
	i := 0
	if !walkShape(root, text, func(s shapeNode) bool {
		if i >= len(want) || s != want[i] {
			return false
		}
		i++
		return true
	}) {
		return false
	}
	return i == len(want)
}

// parseWith parses src and answers the root.
func parseWith(language *ts.Language, src string) (ts.Node, bool) {
	return parse(language, src)
}
