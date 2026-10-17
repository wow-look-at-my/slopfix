package code

import (
	ts "github.com/wow-look-at-my/go-tree-sitter"
	"github.com/wow-look-at-my/slopfix/memo"
)

// named keeps each node's named children, which a rule reads by index.
var named = memo.New[ts.Node, []ts.Node](1 << 14)

// NamedAt answers n.NamedChild(i), a null node past the last, from the
// children read once.
func NamedAt(n ts.Node, i uint32) ts.Node {
	kids := named.Get(n, func() []ts.Node { return NamedChildren(n) })
	if int(i) >= len(kids) {
		return ts.Node{}
	}
	return kids[i]
}

// Children answers every visible child of n, as n.Child(i) answers each.
func Children(n ts.Node) []ts.Node {
	if n.IsNull() {
		return nil
	}
	c := n.Walk()
	if !c.GotoFirstChild() {
		return nil
	}
	var out []ts.Node
	for {
		out = append(out, c.Node())
		if !c.GotoNextSibling() {
			return out
		}
	}
}

// NamedChildren answers every named child of n, as n.NamedChild(i) answers each.
func NamedChildren(n ts.Node) []ts.Node {
	var out []ts.Node
	for _, child := range Children(n) {
		if child.IsNamed() {
			out = append(out, child)
		}
	}
	return out
}
