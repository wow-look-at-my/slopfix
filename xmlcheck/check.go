package xmlcheck

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wow-look-at-my/xml-validator/validator"
)

func (p Problem) String() string {
	if p.Line == 0 {
		return p.Msg
	}
	return fmt.Sprintf("%s:%d: %s", p.File, p.Line, p.Msg)
}

// Run validates every file against the spec's schema, which owns the shape of
// the table, and then holds the trees to the spec's semantic checks. A defect
// of any kind is an error naming it with its file and line. The semantic checks
// read a table whose shape is sound, so they run only after every file passes
// the schema.
func Run(spec Spec, files ...File) error {
	var problems []Problem
	var roots []*Node
	noText := textless(spec.Schema)
	for _, f := range files {
		if spec.Schema != nil {
			if err := validator.ValidateWithSchemaBytes(f.Raw, spec.Schema); err != nil {
				problems = append(problems, schemaProblem(f.Name, err))
				continue
			}
		}
		root, err := Parse(f.Name, f.Raw)
		if err != nil {
			problems = append(problems, Problem{Msg: err.Error()})
			continue
		}
		problems = append(problems, contentProblems(root, noText)...)
		roots = append(roots, root)
	}
	if len(problems) == 0 {
		problems = Check(spec, roots...)
	}
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
	for _, root := range roots {
		c.node(root)
	}
	c.keys(roots)
	c.tests(roots)
	c.shadows(roots)
	return c.problems
}

// schemaProblem words the first defect the schema found in a file.
func schemaProblem(file string, err error) Problem {
	var verr *validator.Error
	if errors.As(err, &verr) && verr.Line > 0 {
		return Problem{File: file, Line: verr.Line, Msg: verr.Message}
	}
	return Problem{Msg: fmt.Sprintf("%s: %v", file, err)}
}

type checker struct {
	spec     Spec
	problems []Problem
}

func (c *checker) add(n *Node, format string, args ...any) {
	c.problems = append(c.problems, Problem{File: n.File, Line: n.Line, Msg: fmt.Sprintf(format, args...)})
}

// node checks the case rule of one element, runs the consumer's own check of
// it, and goes on inside it.
func (c *checker) node(n *Node) {
	if _, ok := n.Attrs[CaseAttr]; ok {
		c.caseAttr(n, c.spec.Elems[n.Name])
	}
	if c.spec.Entry != nil {
		for _, msg := range c.spec.Entry(n, n.Sensitive()) {
			c.add(n, "%s", msg)
		}
	}
	for _, child := range n.Children {
		c.node(child)
	}
}

// caseAttr holds a case= to the elements of this table that take it. The schema
// owns both spellings.
func (c *checker) caseAttr(n *Node, elem Elem) {
	if !elem.Case {
		c.add(n, "<%s> does not take case=", n.Name)
	}
}

// walk visits a tree in document order.
func walk(n *Node, visit func(*Node)) {
	visit(n)
	for _, child := range n.Children {
		walk(child, visit)
	}
}
