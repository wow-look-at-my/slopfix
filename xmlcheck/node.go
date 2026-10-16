// Package xmlcheck validates the XML tables the binary embeds. A table is a
// folder or a file of entries and worked examples. This package reads the file
// into a tree that remembers each line, then holds the tree to a Spec: the
// elements and attributes allowed. This also covers the keys that must stay
// unique, the tests that must not repeat or contradict, and the case rule
// every table shares.
//
// Matching is case-insensitive unless the root or an entry carries
// case="sensitive". The nearest case attribute on the way up from an entry wins.
package xmlcheck

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Both spellings of the case attribute.
const (
	CaseAttr        = "case"
	CaseSensitive   = "sensitive"
	CaseInsensitive = "insensitive"
)

// A Node is one element of a parsed table.
type Node struct {
	File     string
	Line     int
	Name     string
	Attrs    map[string]string
	Text     string
	Children []*Node
	Parent   *Node
}

// Sensitive reports whether the node matches case-sensitively. The nearest
// case attribute on the way up decides, and a table that says nothing is
// insensitive.
func (n *Node) Sensitive() bool {
	for c := n; c != nil; c = c.Parent {
		switch c.Attrs[CaseAttr] {
		case CaseSensitive:
			return true
		case CaseInsensitive:
			return false
		}
	}
	return false
}

// Attr answers an attribute, and whether the element carries it.
func (n *Node) Attr(name string) (string, bool) {
	v, ok := n.Attrs[name]
	return v, ok
}

// Body answers the element's text with the blanks around.
func (n *Node) Body() string { return strings.TrimSpace(n.Text) }

// Where words a position for a message.
func (n *Node) Where() string { return fmt.Sprintf("%s:%d", n.File, n.Line) }

// isRoot reports the document element.
func (n *Node) isRoot() bool { return n.Parent == nil }

var xmlDecl = regexp.MustCompile(`^(\s*<\?xml\s+version=")1\.1(")`)

// Readable answers raw with a declaration encoding/xml accepts.
func Readable(raw []byte) []byte {
	return xmlDecl.ReplaceAll(raw, []byte("${1}1.0${2}"))
}

// Parse reads one table file into a tree.
func Parse(file string, raw []byte) (*Node, error) {
	raw = Readable(raw)
	lines := newLineIndex(raw)
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var root, cur *Node
	for {
		at := dec.InputOffset()
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: does not parse: %w", file, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			node, err := startNode(file, lines.at(at), cur, t)
			if err != nil {
				return nil, err
			}
			if cur == nil {
				if root != nil {
					return nil, fmt.Errorf("%s:%d: a second root element <%s>", file, node.Line, node.Name)
				}
				root = node
			} else {
				cur.Children = append(cur.Children, node)
			}
			cur = node
		case xml.EndElement:
			if cur == nil || cur.Name != t.Name.Local {
				return nil, fmt.Errorf("%s:%d: </%s> closes nothing open", file, lines.at(at), t.Name.Local)
			}
			cur = cur.Parent
		case xml.CharData:
			if cur != nil {
				cur.Text += string(t)
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("%s: holds no element", file)
	}
	if cur != nil {
		return nil, fmt.Errorf("%s: <%s> is never closed", file, cur.Name)
	}
	return root, nil
}

// startNode builds a node from a start tag. The schema-location attributes and
// namespace declarations are not part of a table's vocabulary, so they are
// left out.
func startNode(file string, line int, parent *Node, t xml.StartElement) (*Node, error) {
	node := &Node{File: file, Line: line, Name: t.Name.Local, Attrs: map[string]string{}, Parent: parent}
	for _, a := range t.Attr {
		space, local := a.Name.Space, a.Name.Local
		if space == "xmlns" || space == "xsi" || (space == "" && local == "xmlns") {
			continue
		}
		name := local
		if space != "" {
			name = space + ":" + local
		}
		if _, twice := node.Attrs[name]; twice {
			return nil, fmt.Errorf("%s:%d: <%s> carries the attribute %s twice", file, line, node.Name, name)
		}
		node.Attrs[name] = a.Value
	}
	return node, nil
}

// lineIndex maps a byte offset to a line number.
type lineIndex []int

func newLineIndex(raw []byte) lineIndex {
	var out lineIndex
	for i, b := range raw {
		if b == '\n' {
			out = append(out, i)
		}
	}
	return out
}

// at answers the one-based line of a byte offset.
func (l lineIndex) at(offset int64) int {
	return sort.SearchInts(l, int(offset)) + 1
}
