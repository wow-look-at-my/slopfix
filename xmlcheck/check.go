package xmlcheck

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

func (p Problem) String() string {
	if p.Line == 0 {
		return p.Msg
	}
	return fmt.Sprintf("%s:%d: %s", p.File, p.Line, p.Msg)
}

// Run parses every file and holds the trees to the spec. A defect of any kind
// is an error naming every one of them, each with its file and line.
func Run(spec Spec, files ...File) error {
	var problems []Problem
	var roots []*Node
	for _, f := range files {
		root, err := Parse(f.Name, f.Raw)
		if err != nil {
			problems = append(problems, Problem{Msg: err.Error()})
			continue
		}
		roots = append(roots, root)
	}
	problems = append(problems, Check(spec, roots...)...)
	if len(problems) == 0 {
		return nil
	}
	lines := make([]string, len(problems))
	for i, p := range problems {
		lines[i] = p.String()
	}
	return fmt.Errorf("the XML table is invalid:\n%s", strings.Join(lines, "\n"))
}

// Check holds parsed trees to the spec. The roots form one table: a key, a test
// or an ordered phrase list is read across all of them.
func Check(spec Spec, roots ...*Node) []Problem {
	c := &checker{spec: spec}
	var valid []*Node
	for _, root := range roots {
		if root.Name != spec.Root {
			c.add(root, "the root element is <%s>, and this table's is <%s>", root.Name, spec.Root)
			continue
		}
		if spec.Sensitive && root.Attrs[CaseAttr] != CaseSensitive {
			c.add(root, `<%s> matches text as typed, so its root says case="sensitive"`, root.Name)
		}
		c.node(root)
		valid = append(valid, root)
	}
	c.keys(valid)
	c.tests(valid)
	c.shadows(valid)
	return c.problems
}

type checker struct {
	spec     Spec
	problems []Problem
}

func (c *checker) add(n *Node, format string, args ...any) {
	c.problems = append(c.problems, Problem{File: n.File, Line: n.Line, Msg: fmt.Sprintf(format, args...)})
}

// node checks one element and, when it is sound, everything inside it.
func (c *checker) node(n *Node) {
	elem, known := c.spec.Elems[n.Name]
	if !known {
		c.add(n, "<%s> is not an element of this table", n.Name)
		return
	}
	if p := n.Parent; p != nil && !slices.Contains(c.spec.Elems[p.Name].Children, n.Name) {
		c.add(n, "<%s> may not appear inside <%s>", n.Name, p.Name)
	}
	sound := c.attrs(n, elem)
	if !elem.Text && n.Body() != "" {
		c.add(n, "<%s> may not hold text", n.Name)
		sound = false
	}
	if sound && c.spec.Entry != nil {
		for _, msg := range c.spec.Entry(n, n.Sensitive()) {
			c.add(n, "%s", msg)
		}
	}
	for _, child := range n.Children {
		c.node(child)
	}
}

// attrs holds an element's attributes to its entry in the vocabulary, and
// reports whether it found nothing wrong.
func (c *checker) attrs(n *Node, elem Elem) bool {
	before := len(c.problems)
	allowed := map[string]bool{}
	for _, list := range [][]string{elem.Attrs, elem.Required, elem.Present, elem.Any, {elem.TestAttr, elem.ExpectAttr}} {
		for _, name := range list {
			allowed[name] = true
		}
	}
	names := make([]string, 0, len(n.Attrs))
	for name := range n.Attrs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := n.Attrs[name]
		switch {
		case name == CaseAttr:
			c.caseAttr(n, elem, value)
		case !allowed[name]:
			c.add(n, "<%s> has no attribute %q", n.Name, name)
		default:
			c.enum(n, elem, name, value)
		}
	}
	for _, name := range elem.Required {
		if strings.TrimSpace(n.Attrs[name]) == "" {
			c.add(n, "<%s> needs a %s that is not empty", n.Name, name)
		}
	}
	for _, name := range elem.Present {
		if _, ok := n.Attrs[name]; !ok {
			c.add(n, "<%s> needs a %s", n.Name, name)
		}
	}
	if len(elem.Any) > 0 && !anyFilled(n, elem.Any) {
		c.add(n, "<%s> needs one of %s", n.Name, strings.Join(elem.Any, ", "))
	}
	return len(c.problems) == before
}

func anyFilled(n *Node, names []string) bool {
	for _, name := range names {
		if strings.TrimSpace(n.Attrs[name]) != "" {
			return true
		}
	}
	return false
}

// caseAttr holds the one opt-in spelling to its allowed values.
func (c *checker) caseAttr(n *Node, elem Elem, value string) {
	switch {
	case !elem.Case:
		c.add(n, "<%s> does not take case=", n.Name)
	case value != CaseSensitive && value != CaseInsensitive:
		c.add(n, `case=%q is not "sensitive" or "insensitive"`, value)
	case c.spec.Sensitive && value == CaseInsensitive:
		c.add(n, `<%s> matches text as typed, so it cannot say case="insensitive"`, n.Name)
	}
}

// enum holds an attribute to the values its element allows.
func (c *checker) enum(n *Node, elem Elem, name, value string) {
	allowed, limited := elem.Enum[name]
	if !limited {
		return
	}
	values := []string{value}
	if slices.Contains(elem.Lists, name) {
		values = strings.Fields(value)
	}
	for _, v := range values {
		if !slices.Contains(allowed, v) {
			c.add(n, "%s=%q is not one of %s", name, v, strings.Join(allowed, ", "))
		}
	}
}

// walk visits a tree in document order.
func walk(n *Node, visit func(*Node)) {
	visit(n)
	for _, child := range n.Children {
		walk(child, visit)
	}
}
