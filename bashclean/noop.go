// noop.go drops the statements that run nothing: a bare `true`, `false` or
// `:` beside real work.
//
// Such a statement makes a list or a chain look like work. A pipeline keeps
// its stages, because a stage decides where the stage before it writes.
package bashclean

import (
	"slices"

	"mvdan.cc/sh/v3/syntax"
)

var noopNames = map[string]bool{"true": true, "false": true, ":": true}

// noopName is the no-op a statement runs bare, or "". A redirect, an
// assignment, a negation or a background job does something, so each keeps
// the statement.
func noopName(s *syntax.Stmt) string {
	if s == nil || len(s.Redirs) > 0 || s.Negated || s.Background || s.Coprocess {
		return ""
	}
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok || len(c.Assigns) > 0 || len(c.Args) != 1 {
		return ""
	}
	if n := cmd(c); noopNames[n] {
		return n
	}
	return ""
}

// runsNothing: a no-op, a constant `set`, or constant assignments alone.
func runsNothing(s *syntax.Stmt) bool {
	if noopName(s) != "" {
		return true
	}
	if len(s.Redirs) > 0 || s.Background || s.Coprocess {
		return false
	}
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok {
		return false
	}
	for _, a := range c.Assigns {
		if a.Value != nil && !wordIsConstant(a.Value) {
			return false
		}
	}
	if len(c.Args) == 0 {
		return true
	}
	return cmd(c) == "set" && allConstant(c.Args[1:])
}

func readsStatus(s *syntax.Stmt) bool {
	found := false
	syntax.Walk(s, func(n syntax.Node) bool {
		if p, ok := n.(*syntax.ParamExp); ok && p.Param != nil && p.Param.Value == "?" {
			found = true
		}
		return !found
	})
	return found
}

func succeeds(name string) bool { return name == "true" || name == ":" }

// noopDrop considers only the statements only admits.
type noopDrop struct{ only func(*syntax.Stmt) bool }

func dropNoops(f *syntax.File, only func(*syntax.Stmt) bool) {
	f.Stmts = noopDrop{only}.list(f.Stmts, true)
}

func (d noopDrop) name(s *syntax.Stmt) string {
	n := noopName(s)
	if n == "" || !d.only(s) {
		return ""
	}
	return n
}

// list drops a `true` or `:`. A false ends a `set -e` script, so it stays.
// The last statement of a nested list decides a condition, so it stays.
func (d noopDrop) list(ss []*syntax.Stmt, top bool) []*syntax.Stmt {
	for _, s := range ss {
		d.stmt(s)
	}
	if !slices.ContainsFunc(ss, func(s *syntax.Stmt) bool { return !runsNothing(s) }) {
		return ss
	}
	out := make([]*syntax.Stmt, 0, len(ss))
	for i, s := range ss {
		last := i == len(ss)-1
		switch {
		case !succeeds(d.name(s)):
		case last && !top:
		case !last && readsStatus(ss[i+1]):
		default:
			continue
		}
		out = append(out, s)
	}
	return out
}

func (d noopDrop) stmt(s *syntax.Stmt) {
	if s == nil || s.Cmd == nil {
		return
	}
	switch c := s.Cmd.(type) {
	case *syntax.BinaryCmd:
		d.stmt(c.X)
		d.stmt(c.Y)
		d.chain(s, c)
	case *syntax.Block:
		c.Stmts = d.list(c.Stmts, false)
	case *syntax.Subshell:
		c.Stmts = d.list(c.Stmts, false)
	case *syntax.IfClause:
		for x := c; x != nil; x = x.Else {
			x.Cond = d.list(x.Cond, false)
			x.Then = d.list(x.Then, false)
		}
	case *syntax.WhileClause:
		c.Cond = d.list(c.Cond, false)
		c.Do = d.list(c.Do, false)
	case *syntax.ForClause:
		c.Do = d.list(c.Do, false)
	case *syntax.CaseClause:
		for _, it := range c.Items {
			it.Stmts = d.list(it.Stmts, false)
		}
	case *syntax.TimeClause:
		d.stmt(c.Stmt)
	case *syntax.FuncDecl:
		d.stmt(c.Body)
	}
}

// chain keeps whether the chain succeeds. A side that never runs goes with
// the no-op beside it.
func (d noopDrop) chain(s *syntax.Stmt, b *syntax.BinaryCmd) {
	x, y := d.name(b.X), d.name(b.Y)
	switch b.Op {
	case syntax.AndStmt:
		switch {
		case x == "false":
			promoteInto(s, b.X)
		case x != "":
			promoteInto(s, b.Y)
		case succeeds(y):
			promoteInto(s, b.X)
		}
	case syntax.OrStmt:
		switch {
		case x == "false":
			promoteInto(s, b.Y)
		case x != "":
			promoteInto(s, b.X)
		case y == "false":
			promoteInto(s, b.X)
		}
	}
}

// runsNoWork: no statement in the tree runs anything but a no-op, a constant
// `set` or a constant assignment.
func runsNoWork(f *syntax.File) bool {
	work := false
	syntax.Walk(f, func(n syntax.Node) bool {
		s, ok := n.(*syntax.Stmt)
		if work || !ok {
			return !work
		}
		switch s.Cmd.(type) {
		case nil:
			work = len(s.Redirs) > 0
		case *syntax.CallExpr:
			work = !runsNothing(s)
		case *syntax.BinaryCmd, *syntax.Block, *syntax.Subshell, *syntax.IfClause,
			*syntax.WhileClause, *syntax.ForClause, *syntax.CaseClause, *syntax.TimeClause:
		default:
			work = true
		}
		return !work
	})
	return !work
}

func noopRemove(f *syntax.File) {
	dropNoops(f, func(*syntax.Stmt) bool { return true })
}
