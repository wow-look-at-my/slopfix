package bashclean

import (
	"regexp"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Result describes the parsed cleanup decision. A denied command is returned
// unchanged; Rules contains the stable rule names used by the shell hook.
type Result struct {
	Command string
	Denied  bool
	Reason  string
	Changed bool
	Rules   []string
}

// Transform parses command, applies the cleanup-bash-cmds rules, and prints it
// using mvdan's shell printer. Parse failures fail open.
func Transform(command string) Result {
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return Result{Command: command}
	}
	if hasHeredoc(f) {
		return Result{Command: command, Denied: true, Reason: "heredoc", Rules: []string{"heredoc"}}
	}
	if hasStatementCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		return ok && regexp.MustCompile(`^perl[0-9.]*$`).MatchString(e.name)
	}) {
		return deny(command, "perl")
	}
	if hasFileRead(f) {
		return deny(command, "file_read")
	}
	if hasStatementCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		return ok && (e.name == "shred" || e.name == "srm")
	}) {
		return deny(command, "shred")
	}
	if hasGitRM(f) {
		return deny(command, "git_rm")
	}
	if hasBadTruncate(f) {
		return deny(command, "truncate_zero")
	}
	if hasBadRM(f) {
		return deny(command, "rm_flag")
	}
	before := printFile(f)
	rules := []string{}
	apply := func(name string, fn func(*syntax.File)) {
		b := printFile(f)
		fn(f)
		if printFile(f) != b {
			rules = appendUnique(rules, name)
		}
	}
	apply("devnull", func(f *syntax.File) { walkCalls(f, func(c *syntax.CallExpr) { scrub(c) }) })
	apply("docker_compose_restart", func(f *syntax.File) { walkCalls(f, dockerCompose) })
	apply("gh_wait_ci", func(f *syntax.File) { walkCalls(f, ghWaitCI) })
	apply("rm_recycle", func(f *syntax.File) { walkCalls(f, rewriteRM) })
	apply("truncate_recycle", func(f *syntax.File) { walkCalls(f, rewriteTruncate) })
	apply("find_delete_recycle", func(f *syntax.File) { walkCalls(f, rewriteFind) })
	apply("head_tail", func(f *syntax.File) {
		trailing(f, func(s *syntax.Stmt) { stripStages(s, map[string]bool{"head": true, "tail": true}) })
	})
	apply("or_true", func(f *syntax.File) { trailing(f, stripOrTrue) })
	apply("grep", func(f *syntax.File) {
		trailing(f, func(s *syntax.Stmt) { stripStages(s, map[string]bool{"grep": true}) })
	})
	apply("stderr_merge", func(f *syntax.File) { trailing(f, stripMerge) })
	apply("tee", func(f *syntax.File) { trailing(f, tee) })
	apply("sleep_cap", func(f *syntax.File) { walkCalls(f, capSleep) })
	apply("narration_remove", func(f *syntax.File) { narration(f) })
	apply("pipefail", ensurePipefail)
	after := printFile(f)
	return Result{Command: after, Changed: before != after, Rules: rules}
}

// Clean is a concise alias for Transform.
func Clean(command string) Result { return Transform(command) }
func deny(command, reason string) Result {
	return Result{Command: command, Denied: true, Reason: reason, Rules: []string{reason}}
}
func appendUnique(a []string, s string) []string {
	for _, x := range a {
		if x == s {
			return a
		}
	}
	return append(a, s)
}
func printFile(f *syntax.File) string {
	var b strings.Builder
	_ = syntax.NewPrinter().Print(&b, f)
	return b.String()
}
func word(v string) *syntax.Word {
	return &syntax.Word{Parts: []syntax.WordPart{&syntax.Lit{Value: v}}}
}
func literal(w *syntax.Word) (string, bool)  { return wordLiteral(w) }
func args(c *syntax.CallExpr) []*syntax.Word { return c.Args }
func cmd(c *syntax.CallExpr) string {
	if len(c.Args) == 0 {
		return ""
	}
	s, ok := literal(c.Args[0])
	if !ok {
		return ""
	}
	return strings.TrimPrefix(s, `\`)
}

func walkCalls(n syntax.Node, fn func(*syntax.CallExpr)) {
	syntax.Walk(n, func(n syntax.Node) bool {
		if c, ok := n.(*syntax.CallExpr); ok {
			fn(c)
		}
		return true
	})
}
func hasHeredoc(n syntax.Node) bool {
	found := false
	syntax.Walk(n, func(n syntax.Node) bool {
		if r, ok := n.(*syntax.Redirect); ok && (r.Op == syntax.Hdoc || r.Op == syntax.DashHdoc) {
			found = true
		}
		return !found
	})
	return found
}
func hasStatementCall(f *syntax.File, p func(*syntax.CallExpr) bool) bool {
	hit := false
	var st func(*syntax.Stmt)
	st = func(s *syntax.Stmt) {
		if s == nil || hit {
			return
		}
		switch c := s.Cmd.(type) {
		case *syntax.CallExpr:
			hit = p(c)
		case *syntax.BinaryCmd:
			st(c.X)
			st(c.Y)
		case *syntax.Block:
			for _, x := range c.Stmts {
				st(x)
			}
		case *syntax.Subshell:
			for _, x := range c.Stmts {
				st(x)
			}
		case *syntax.IfClause:
			for _, x := range c.Cond {
				st(x)
			}
			for _, x := range c.Then {
				st(x)
			}
			if c.Else != nil {
				st(&syntax.Stmt{Cmd: c.Else})
			}
		case *syntax.WhileClause:
			for _, x := range c.Cond {
				st(x)
			}
			for _, x := range c.Do {
				st(x)
			}
		case *syntax.ForClause:
			for _, x := range c.Do {
				st(x)
			}
		case *syntax.FuncDecl:
			st(c.Body)
		}
	}
	for _, s := range f.Stmts {
		st(s)
	}
	return hit
}

func isWord(w *syntax.Word, v string) bool { s, ok := literal(w); return ok && s == v }
func allStatic(ws []*syntax.Word) bool {
	for _, w := range ws {
		if _, ok := literal(w); !ok {
			return false
		}
	}
	return true
}
func replaceArgs(c *syntax.CallExpr, i int, ws ...*syntax.Word) {
	c.Args = append(append(append([]*syntax.Word{}, c.Args[:i]...), ws...), c.Args[i+1:]...)
}
func scrub(c *syntax.CallExpr) {
	for i := 0; i < len(c.Args); i++ {
		_ = i
	} /* redirects are attached to Stmt and handled by the AST walk below */
}

func dockerCompose(c *syntax.CallExpr) {
	if len(c.Args) >= 3 && isWord(c.Args[0], "docker") && isWord(c.Args[1], "compose") && isWord(c.Args[2], "restart") {
		c.Args = append([]*syntax.Word{word("docker"), word("compose"), word("up"), word("-d"), word("--force-recreate")}, c.Args[3:]...)
	}
}
func ghWaitCI(c *syntax.CallExpr) {
	if len(c.Args) < 3 || !isWord(c.Args[0], "gh") {
		return
	}
	a := c.Args
	g, _ := literal(a[1])
	v, _ := literal(a[2])
	if g == "run" && (v == "view" || v == "watch" || v == "rerun") && len(a) >= 4 {
		id, ok := literal(a[3])
		if !ok || !regexp.MustCompile(`^[0-9]+$`).MatchString(id) {
			return
		}
		sub := v
		if v == "view" {
			sub = "view"
		}
		if v == "watch" {
			c.Args = []*syntax.Word{word("gh"), word("wait-ci"), a[3]}
			return
		}
		if v == "view" {
			for _, w := range a[4:] {
				x, _ := literal(w)
				if x == "--log-failed" {
					sub = "log"
					c.Args = []*syntax.Word{word("gh"), word("wait-ci"), word("log"), a[3], word("--failed")}
					return
				}
				if x == "--log" {
					sub = "log"
					c.Args = []*syntax.Word{word("gh"), word("wait-ci"), word("log"), a[3]}
					return
				}
			}
		}
		c.Args = append([]*syntax.Word{word("gh"), word("wait-ci"), word(sub), a[3]}, a[4:]...)
		return
	}
	if (g == "run" && v == "list") || (g == "pr" && v == "checks") {
		for _, w := range a[3:] {
			x, _ := literal(w)
			if !strings.HasPrefix(x, "-") {
				return
			}
		}
		sub := "runs"
		if g == "pr" {
			sub = "checks"
		}
		c.Args = append([]*syntax.Word{word("gh"), word("wait-ci"), word(sub)}, a[3:]...)
	}
}

func capSleep(c *syntax.CallExpr) {
	if cmd(c) != "sleep" {
		return
	}
	if len(c.Args) < 2 {
		c.Args = []*syntax.Word{c.Args[0], word("3")}
		return
	}
	total := 0.0
	for _, w := range c.Args[1:] {
		s, ok := literal(w)
		if !ok {
			return
		}
		m := regexp.MustCompile(`^([0-9]+(?:\.[0-9]*)?|\.[0-9]+)([smhd]?)$`).FindStringSubmatch(s)
		if m == nil {
			c.Args = []*syntax.Word{c.Args[0], word("3")}
			return
		}
		n, _ := strconv.ParseFloat(m[1], 64)
		for _, u := range []struct {
			s string
			v float64
		}{{"s", 1}, {"m", 60}, {"h", 3600}, {"d", 86400}} {
			if m[2] == u.s {
				n *= u.v
			}
		}
		total += n
	}
	if total > 3 {
		c.Args = []*syntax.Word{c.Args[0], word("3")}
	}
}

func rewriteRM(c *syntax.CallExpr) {
	e, ok := effectiveCommand(c)
	if !ok || e.name != "rm" {
		return
	}
	out := []*syntax.Word{word("recycler"), word("trash")}
	for _, w := range c.Args[e.index+1:] {
		s, static := literal(w)
		if static && (s == "--" || strings.HasPrefix(s, "-") && s != "-") {
			continue
		}
		out = append(out, w)
	}
	c.Args = append(append(append([]*syntax.Word{}, c.Args[:e.index]...), out...), []*syntax.Word{}...)
}
func rewriteTruncate(c *syntax.CallExpr) {
	e, ok := effectiveCommand(c)
	if !ok || e.name != "truncate" {
		return
	}
	var out []*syntax.Word
	zero := false
	for i := e.index + 1; i < len(c.Args); i++ {
		s, st := literal(c.Args[i])
		if st && (s == "-s0" || s == "--size=0") {
			zero = true
			continue
		}
		if st && (s == "-s" || s == "--size") && i+1 < len(c.Args) {
			x, _ := literal(c.Args[i+1])
			if x == "0" {
				zero = true
				i++
				continue
			}
		}
		out = append(out, c.Args[i])
	}
	if zero && len(out) > 0 {
		c.Args = append(append(append([]*syntax.Word{}, c.Args[:e.index]...), word("recycler"), word("trash")), out...)
	}
}
func rewriteFind(c *syntax.CallExpr) {
	e, ok := effectiveCommand(c)
	if !ok || e.name != "find" {
		return
	}
	out := []*syntax.Word{}
	for _, w := range c.Args[e.index+1:] {
		if isWord(w, "-delete") {
			out = append(out, word("-exec"), word("recycler"), word("trash"), word("{}"), word("+"))
		} else {
			out = append(out, w)
		}
	}
	c.Args = append(append([]*syntax.Word{}, c.Args[:e.index+1]...), out...)
}
