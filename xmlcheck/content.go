// xsdNode is one node of a schema, read only as far as the content model of the elements it declares.

package xmlcheck

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// xsdNode is one node of a schema, read only as far as the content model of
// the elements it declares.
type xsdNode struct {
	XMLName  xml.Name
	Name     string    `xml:"name,attr"`
	Type     string    `xml:"type,attr"`
	Mixed    string    `xml:"mixed,attr"`
	Children []xsdNode `xml:",any"`
}

// textless answers the element names whose schema declares an element-only
// content. The validator this package calls holds a table to its elements and
// its attributes and leaves the characters it carries unchecked. The text of an
// element is judged here.
//
// An element the schema does not declare plainly is left to take text: a
// reference. A type this reader cannot resolve, and declarations that
// disagree all answer text as allowed.
func textless(schema []byte) set.Set[string] {
	out := set.Of[string]()
	if len(schema) == 0 {
		return out
	}
	var root xsdNode
	if err := xml.Unmarshal(Readable(schema), &root); err != nil {
		return out
	}
	types := map[string]bool{}
	walkXSD(root, func(n xsdNode) {
		if n.XMLName.Local == "complexType" && n.Name != "" {
			types[n.Name] = typeTakesText(n)
		}
	})
	takes := map[string]bool{}
	declared := set.Of[string]()
	walkXSD(root, func(n xsdNode) {
		if n.XMLName.Local != "element" || n.Name == "" {
			return
		}
		here := elementTakesText(n, types)
		if declared.Contains(n.Name) {
			takes[n.Name] = takes[n.Name] || here
			return
		}
		declared.Add(n.Name)
		takes[n.Name] = here
	})
	for name, ok := range takes {
		if !ok {
			out.Add(name)
		}
	}
	return out
}

// elementTakesText reports an element declaration whose content carries text.
func elementTakesText(n xsdNode, types map[string]bool) bool {
	for i, child := range n.Children {
		switch child.XMLName.Local {
		case "complexType":
			return typeTakesText(n.Children[i])
		case "simpleType":
			return true
		}
	}
	if n.Type == "" {
		return true
	}
	takes, known := types[localName(n.Type)]
	return !known || takes
}

// typeTakesText reports a complex type that carries text of its own. A simple
// or a complex content says so; a type declaring only elements and attributes
// does not.
func typeTakesText(t xsdNode) bool {
	if t.Mixed == "true" {
		return true
	}
	takes := false
	walkXSD(t, func(n xsdNode) {
		switch n.XMLName.Local {
		case "simpleContent", "complexContent":
			takes = true
		}
	})
	return takes
}

// localName drops the namespace prefix of a qualified type name.
func localName(name string) string {
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		return name[i+1:]
	}
	return name
}

func walkXSD(n xsdNode, visit func(xsdNode)) {
	visit(n)
	for _, child := range n.Children {
		walkXSD(child, visit)
	}
}

// contentProblems reports text an element carries where its schema declares an
// element-only content.
func contentProblems(root *Node, noText set.Set[string]) []Problem {
	var out []Problem
	walk(root, func(n *Node) {
		if !noText.Contains(n.Name) || strings.TrimSpace(n.Text) == "" {
			return
		}
		out = append(out, Problem{File: n.File, Line: n.Line, Msg: fmt.Sprintf("<%s> takes no text: its schema declares an element-only content", n.Name)})
	})
	return out
}
