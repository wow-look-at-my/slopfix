package bashclean

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// splitGitC lifts every `-C dir` out of a git call's global options. A
// permission rule reads `git push`, and `-C dir` would stand between both
// words. The returned args are the call without them.
func splitGitC(c *syntax.CallExpr) (dirs, args []*syntax.Word, ok bool) {
	e, found := effectiveCommand(c)
	if !found || e.name != "git" {
		return nil, nil, false
	}
	args = append([]*syntax.Word{}, c.Args[:e.index+1]...)
	rest := c.Args[e.index+1:]
	i := 0
	for ; i < len(rest); i++ {
		s, st := literal(rest[i])
		if !st || !strings.HasPrefix(s, "-") {
			break
		}
		if s == "-C" {
			if i+1 >= len(rest) {
				return nil, nil, false
			}
			dirs = append(dirs, rest[i+1])
			i++
			continue
		}
		args = append(args, rest[i])
		if s == "-c" || s == "--git-dir" || s == "--work-tree" || s == "--namespace" {
			if i+1 >= len(rest) {
				return nil, nil, false
			}
			args = append(args, rest[i+1])
			i++
		}
	}
	if len(dirs) == 0 {
		return nil, nil, false
	}
	return dirs, append(args, rest[i:]...), true
}

// cdChain is `cd d1 && cd d2 && call`, left-associative as the parser builds it.
func cdChain(dirs []*syntax.Word, call *syntax.CallExpr) *syntax.BinaryCmd {
	var x *syntax.Stmt
	for _, d := range dirs {
		cd := &syntax.Stmt{Cmd: &syntax.CallExpr{Args: []*syntax.Word{word("cd"), d}}}
		if x == nil {
			x = cd
			continue
		}
		x = &syntax.Stmt{Cmd: &syntax.BinaryCmd{Op: syntax.AndStmt, X: x, Y: cd}}
	}
	return &syntax.BinaryCmd{Op: syntax.AndStmt, X: x, Y: &syntax.Stmt{Cmd: call}}
}

// rewriteGitC writes `git -C dir args` as `cd dir && git args`. The cd stays
// bare only on the command's trailing leaf, where nothing runs after it. A
// call anywhere else, or one with a redirect or a flag on its statement, gets
// a subshell. The directory change reaches the git call alone.
func rewriteGitC(f *syntax.File) {
	var tail *syntax.Stmt
	if len(f.Stmts) > 0 {
		tail = spineLeaf(f.Stmts[len(f.Stmts)-1])
	}
	syntax.Walk(f, func(n syntax.Node) bool {
		s, ok := n.(*syntax.Stmt)
		if !ok {
			return true
		}
		c, ok := s.Cmd.(*syntax.CallExpr)
		if !ok {
			return true
		}
		dirs, args, ok := splitGitC(c)
		if !ok {
			return true
		}
		chain := cdChain(dirs, &syntax.CallExpr{Assigns: c.Assigns, Args: args})
		if s == tail && len(s.Redirs) == 0 && !s.Negated && !s.Background && !s.Coprocess {
			s.Cmd = chain
			return false
		}
		s.Cmd = &syntax.Subshell{Stmts: []*syntax.Stmt{{Cmd: chain}}}
		return false
	})
}
