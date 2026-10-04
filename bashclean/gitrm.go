package bashclean

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// gitRM is a `git rm` call that deletes working-tree files.
type gitRM struct {
	head     []*syntax.Word // wrappers, `git`, its global options and `rm`
	inDir    bool           // a `-C` names another directory; rewriteGitC turns it into a `cd` first
	opts     []*syntax.Word
	paths    []*syntax.Word
	fromFile bool
}

// parseGitRM: the SUBCOMMAND is the leading non-flag word after `git`, so
// `git -c k=v rm f` counts while `git commit -m rm` does not. `--cached` and a
// dry run delete nothing, so they do not count.
func parseGitRM(c *syntax.CallExpr) (gitRM, bool) {
	e, ok := effectiveCommand(c)
	if !ok || e.name != "git" {
		return gitRM{}, false
	}
	args := c.Args[e.index+1:]
	var g gitRM
	i := 0
	for ; i < len(args); i++ {
		s, st := literal(args[i])
		if !st {
			return gitRM{}, false
		}
		if s == "-C" || s == "-c" {
			if i+1 >= len(args) {
				return gitRM{}, false
			}
			g.inDir = g.inDir || s == "-C"
			i++
			continue
		}
		if strings.HasPrefix(s, "-") {
			continue
		}
		if s != "rm" {
			return gitRM{}, false
		}
		break
	}
	if i >= len(args) {
		return gitRM{}, false
	}
	g.head = c.Args[:e.index+1+i+1]
	done := false
	for _, w := range args[i+1:] {
		s, st := literal(w)
		switch {
		case done || !st:
			g.paths = append(g.paths, w)
		case s == "--":
			done = true
		case s == "--cached" || s == "--dry-run" || isShortDryRun(s):
			return gitRM{}, false
		case strings.HasPrefix(s, "--pathspec-from-file"):
			g.fromFile = true
		case strings.HasPrefix(s, "-") && len(s) > 1:
			g.opts = append(g.opts, w)
		default:
			g.paths = append(g.paths, w)
		}
	}
	return g, len(g.paths) > 0 || g.fromFile
}

func isShortDryRun(s string) bool {
	return len(s) > 1 && s[0] == '-' && s[1] != '-' && strings.ContainsRune(s[1:], 'n')
}

// hasUnrewritableGitRM: a pathspec file hides the paths, so no rewrite can
// name what to recycle.
func hasUnrewritableGitRM(f *syntax.File) bool {
	return hasStatementCall(f, func(c *syntax.CallExpr) bool {
		g, ok := parseGitRM(c)
		return ok && g.fromFile
	})
}

// rewriteGitRM unstages first and recycles second. A path already gone from
// disk then still gets its deletion staged, and a path git does not track
// stops the chain before anything moves.
func rewriteGitRM(f *syntax.File) {
	syntax.Walk(f, func(n syntax.Node) bool {
		s, ok := n.(*syntax.Stmt)
		if !ok {
			return true
		}
		c, ok := s.Cmd.(*syntax.CallExpr)
		if !ok {
			return true
		}
		g, ok := parseGitRM(c)
		if !ok || g.fromFile || g.inDir {
			return true
		}
		repl, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(g.replacement()), "")
		if err != nil || len(repl.Stmts) != 1 {
			return true
		}
		bin, ok := repl.Stmts[0].Cmd.(*syntax.BinaryCmd)
		if !ok {
			return true
		}
		if call, ok := bin.X.Cmd.(*syntax.CallExpr); ok {
			call.Assigns = c.Assigns
		}
		s.Cmd = bin
		return false
	})
}

func (g gitRM) replacement() string {
	git := append(append(append(append([]*syntax.Word{}, g.head...), word("--cached")), g.opts...), word("--"))
	git = append(git, g.paths...)
	trash := append([]*syntax.Word{word("recycler"), word("trash")}, g.paths...)
	if needsSeparator(g.paths) {
		trash = append([]*syntax.Word{word("recycler"), word("trash"), word("--")}, g.paths...)
	}
	return printWords(git) + " && " + printWords(trash)
}

func printWords(ws []*syntax.Word) string {
	parts := make([]string, len(ws))
	for i, w := range ws {
		var b strings.Builder
		_ = syntax.NewPrinter().Print(&b, w)
		parts[i] = b.String()
	}
	return strings.Join(parts, " ")
}
