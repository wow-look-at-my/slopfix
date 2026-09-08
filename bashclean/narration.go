// narration.go removes constant narration.
//
// An echo or a printf becomes the no-op `:` only when its stdout REACHES THE
// TERMINAL. `X=$(echo hi)`, `echo x | jq` and `echo x > f` are data, not
// narration, and a function body's call site decides its visibility rather than
// the body. Visibility threads top-down, so this traversal is hand-rolled over
// statement structure and never enters Word parts, which excludes every capture
// by construction.
//
// The whole command is replaced rather than deleted, so the statement count
// never drops and the statement still succeeds.
package bashclean

import (
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

var globRisk = regexp.MustCompile(`[*?\[{]`)

// wordIsConstant: a Lit with no glob or tilde risk, a single-quoted string, or
// a double-quoted string over Lits.
func wordIsConstant(w *syntax.Word) bool {
	if w == nil {
		return false
	}
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			if globRisk.MatchString(x.Value) || strings.HasPrefix(x.Value, "~") {
				return false
			}
		case *syntax.SglQuoted:
		case *syntax.DblQuoted:
			for _, q := range x.Parts {
				if _, ok := q.(*syntax.Lit); !ok {
					return false
				}
			}
		default:
			return false
		}
	}
	return true
}

// redirsStderrOnly: every redirect leaves stdout alone. An unknown op fails
// closed into leaving the echo alone.
func redirsStderrOnly(s *syntax.Stmt) bool {
	for _, r := range s.Redirs {
		if r.N == nil || r.N.Value != "2" {
			return false
		}
		if r.Op != syntax.RdrOut && r.Op != syntax.AppOut && r.Op != syntax.DplOut {
			return false
		}
	}
	return true
}

func isNarration(c *syntax.CallExpr) bool {
	e, ok := effectiveCommand(c)
	if !ok {
		return false
	}
	rest := c.Args[e.index+1:]
	switch e.name {
	case "echo":
		return allConstant(rest)
	case "printf":
		return len(rest) > 0 && allConstant(rest)
	}
	return false
}

func allConstant(ws []*syntax.Word) bool {
	for _, w := range ws {
		if !wordIsConstant(w) {
			return false
		}
	}
	return true
}

func narration(f *syntax.File) {
	for _, s := range f.Stmts {
		narrationStmt(s, true)
	}
}

func narrationStmt(s *syntax.Stmt, vis bool) {
	if s == nil || s.Cmd == nil {
		return
	}
	v := vis && redirsStderrOnly(s) && !s.Coprocess
	switch c := s.Cmd.(type) {
	case *syntax.CallExpr:
		if v && isNarration(c) {
			s.Cmd = &syntax.CallExpr{Args: []*syntax.Word{word(":")}}
		}
	case *syntax.BinaryCmd:
		switch c.Op {
		case syntax.Pipe, syntax.PipeAll:
			narrationStmt(c.X, false)
			narrationStmt(c.Y, v)
		case syntax.AndStmt, syntax.OrStmt:
			narrationStmt(c.X, v)
			narrationStmt(c.Y, v)
		}
	case *syntax.Block:
		narrationStmts(c.Stmts, v)
	case *syntax.Subshell:
		narrationStmts(c.Stmts, v)
	case *syntax.WhileClause:
		narrationStmts(c.Cond, v)
		narrationStmts(c.Do, v)
	case *syntax.ForClause:
		narrationStmts(c.Do, v)
	case *syntax.IfClause:
		for x := c; x != nil; x = x.Else {
			narrationStmts(x.Cond, v)
			narrationStmts(x.Then, v)
		}
	case *syntax.CaseClause:
		for _, it := range c.Items {
			narrationStmts(it.Stmts, v)
		}
	case *syntax.TimeClause:
		narrationStmt(c.Stmt, v)
	}
}

func narrationStmts(ss []*syntax.Stmt, vis bool) {
	for _, s := range ss {
		narrationStmt(s, vis)
	}
}
