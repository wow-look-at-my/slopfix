package bashclean

import (
	"regexp"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Result is the parsed cleanup decision. A denied command is returned
// unchanged; Rules names the rules that fired.
type Result struct {
	Command string
	Denied  bool
	Reason  string
	Changed bool
	Rules   []string
}

var perlName = regexp.MustCompile(`^perl[0-9.]*$`)

// Transform parses command, applies the cleanup-bash-cmds rules, and prints it
// using mvdan's shell printer. Parse failures fail open.
func Transform(command string) Result {
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return Result{Command: command}
	}
	if hasHeredoc(f) {
		return deny(command, "heredoc")
	}
	if hasStatementCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		return ok && perlName.MatchString(e.name)
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
	// The rewrite runs to a fixed point: one rule's output is another rule's
	// input, and a single pass leaves that second rewrite undone.
	for i := 0; i < 20; i++ {
		pass := printFile(f)
		onePass(apply)
		if printFile(f) == pass {
			break
		}
	}
	apply("pipefail", ensurePipefail)
	after := printFile(f)
	return Result{Command: after, Changed: before != after, Rules: rules}
}

func onePass(apply func(string, func(*syntax.File))) {
	apply("devnull", scrubDevnull)
	apply("docker_compose_restart", func(f *syntax.File) { walkCalls(f, dockerCompose) })
	apply("gh_wait_ci", func(f *syntax.File) { walkCalls(f, ghWaitCI) })
	apply("rm_recycle", func(f *syntax.File) { walkCalls(f, rewriteRM) })
	apply("truncate_recycle", func(f *syntax.File) { walkCalls(f, rewriteTruncate) })
	apply("find_delete_recycle", func(f *syntax.File) { walkCalls(f, rewriteFind) })
	apply("head_tail", func(f *syntax.File) {
		trailing(f, func(s *syntax.Stmt) { stripStages(spineLeaf(s), isHeadTailStage) })
	})
	apply("or_true", func(f *syntax.File) { trailing(f, stripOrTrue) })
	apply("grep", func(f *syntax.File) {
		trailing(f, func(s *syntax.Stmt) { stripStages(spineLeaf(s), isGrepStage) })
	})
	apply("stderr_merge", func(f *syntax.File) {
		trailing(f, func(s *syntax.Stmt) { stripMerge(lastStage(spineLeaf(s))) })
	})
	apply("tee", func(f *syntax.File) { trailing(f, func(s *syntax.Stmt) { teeRewrite(spineLeaf(s)) }) })
	apply("sleep_cap", func(f *syntax.File) { walkCalls(f, capSleep) })
	apply("narration_remove", narration)
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
		if s == nil || hit || s.Cmd == nil {
			return
		}
		switch c := s.Cmd.(type) {
		case *syntax.CallExpr:
			if p(c) {
				hit = true
			}
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
		case *syntax.CaseClause:
			for _, it := range c.Items {
				for _, x := range it.Stmts {
					st(x)
				}
			}
		case *syntax.TimeClause:
			st(c.Stmt)
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

// ---------------------------------------------------------------------------
// Scrub the stderr discard, tree-wide. stderr is how a command reports its own
// failure, so a discarded stderr turns one command into two: the command, and
// a second call asking whether it worked.
//
//	2>/dev/null, 2>>/dev/null   dropped
//	&>/dev/null, &>>/dev/null   demoted to >/dev/null, >>/dev/null
//	>&/dev/null                 demoted to >/dev/null
//	>/dev/null 2>&1             the 2>&1 goes, the >/dev/null stays
//
// A bare 2>&1 is a MERGE and survives. It is dropped only when an EARLIER
// entry in the same list already sent stdout to /dev/null, so the reversed
// `2>&1 >/dev/null`, which sends stderr to the terminal, is left alone.
// ---------------------------------------------------------------------------

func isDevnull(w *syntax.Word) bool { return isWord(w, "/dev/null") }

func isStderrDevnull(r *syntax.Redirect) bool {
	return r.N != nil && r.N.Value == "2" &&
		(r.Op == syntax.RdrOut || r.Op == syntax.AppOut) && isDevnull(r.Word)
}

func isAllDevnull(r *syntax.Redirect) bool {
	return r.N == nil &&
		(r.Op == syntax.RdrAll || r.Op == syntax.AppAll || r.Op == syntax.DplOut) && isDevnull(r.Word)
}

func isStdoutDevnull(r *syntax.Redirect) bool {
	return (r.N == nil || r.N.Value == "1") &&
		(r.Op == syntax.RdrOut || r.Op == syntax.AppOut) && isDevnull(r.Word)
}

func isStderrToStdout(r *syntax.Redirect) bool {
	return r.N != nil && r.N.Value == "2" && r.Op == syntax.DplOut && isWord(r.Word, "1")
}

// One positional pass over a Redirs list: order decides whether a trailing
// 2>&1 lands in /dev/null or on the terminal.
func scrubRedirs(rs []*syntax.Redirect) []*syntax.Redirect {
	out := make([]*syntax.Redirect, 0, len(rs))
	sawNull := false
	for _, r := range rs {
		switch {
		case isStderrDevnull(r):
		case isAllDevnull(r):
			if r.Op == syntax.AppAll {
				r.Op = syntax.AppOut
			} else {
				r.Op = syntax.RdrOut
			}
			out = append(out, r)
			sawNull = true
		case isStdoutDevnull(r):
			out = append(out, r)
			sawNull = true
		case sawNull && isStderrToStdout(r):
		default:
			out = append(out, r)
		}
	}
	return out
}

func scrubDevnull(f *syntax.File) {
	syntax.Walk(f, func(n syntax.Node) bool {
		if s, ok := n.(*syntax.Stmt); ok && len(s.Redirs) > 0 {
			s.Redirs = scrubRedirs(s.Redirs)
		}
		return true
	})
}

func dockerCompose(c *syntax.CallExpr) {
	if len(c.Args) >= 3 && isWord(c.Args[0], "docker") && isWord(c.Args[1], "compose") && isWord(c.Args[2], "restart") {
		c.Args = append([]*syntax.Word{word("docker"), word("compose"), word("up"), word("-d"), word("--force-recreate")}, c.Args[3:]...)
	}
}

var runID = regexp.MustCompile(`^[0-9]+$`)

// onlyFlagsAndValues asks whether no remaining word is a POSITIONAL. A dash
// word is a flag, and a bare word directly after a dash word is that flag's
// value (`--branch main`). A `--flag=value` form carries its own value, so the
// word after it is a positional again. `gh pr checks 42` names one pull
// request where `gh wait-ci checks` reads the current branch, which is a
// different question, so a positional blocks the rewrite.
func onlyFlagsAndValues(ws []*syntax.Word) bool {
	for i, w := range ws {
		s, _ := literal(w)
		if strings.HasPrefix(s, "-") {
			continue
		}
		if i == 0 {
			return false
		}
		prev, _ := literal(ws[i-1])
		if !strings.HasPrefix(prev, "-") || strings.Contains(prev, "=") {
			return false
		}
	}
	return true
}

func withoutFlag(ws []*syntax.Word, name string) []*syntax.Word {
	out := []*syntax.Word{}
	for _, w := range ws {
		if s, ok := literal(w); ok && s == name {
			continue
		}
		out = append(out, w)
	}
	return out
}

func hasFlag(ws []*syntax.Word, name string) bool {
	for _, w := range ws {
		if s, ok := literal(w); ok && s == name {
			return true
		}
	}
	return false
}

func ghWaitCI(c *syntax.CallExpr) {
	a := c.Args
	if len(a) < 3 || !isWord(a[0], "gh") {
		return
	}
	set := func(head, rest []*syntax.Word) {
		c.Args = append(append([]*syntax.Word{word("gh"), word("wait-ci")}, head...), rest...)
	}
	g, _ := literal(a[1])
	v, _ := literal(a[2])
	var tail []*syntax.Word
	isID := false
	if len(a) >= 4 {
		tail = a[4:]
		if s, ok := literal(a[3]); ok {
			isID = runID.MatchString(s)
		}
	}
	switch {
	case g == "run" && v == "view" && isID:
		switch {
		case hasFlag(tail, "--log-failed"):
			set([]*syntax.Word{word("log"), a[3], word("--failed")}, withoutFlag(tail, "--log-failed"))
		case hasFlag(tail, "--log"):
			set([]*syntax.Word{word("log"), a[3]}, withoutFlag(tail, "--log"))
		default:
			set([]*syntax.Word{word("view"), a[3]}, tail)
		}
	case g == "run" && v == "watch" && isID:
		set([]*syntax.Word{a[3]}, tail)
	case g == "run" && v == "rerun" && isID:
		set([]*syntax.Word{word("rerun"), a[3]}, tail)
	case g == "run" && v == "list" && onlyFlagsAndValues(a[3:]):
		set([]*syntax.Word{word("runs")}, a[3:])
	case g == "pr" && v == "checks" && onlyFlagsAndValues(a[3:]):
		set([]*syntax.Word{word("checks")}, a[3:])
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

// rmTargets returns the real targets of an `rm` call. `--` ends flag parsing,
// and it is RE-EMITTED in front of a dash-leading name, so `rm -- -weirdname`
// becomes `recycler trash -- -weirdname` and the filename is not re-read as a
// recycler flag. Dropping the separator would change which file is deleted.
func rmTargets(args []*syntax.Word) []*syntax.Word {
	ops := []*syntax.Word{}
	done := false
	for _, w := range args {
		if done {
			ops = append(ops, w)
			continue
		}
		s, ok := literal(w)
		switch {
		case !ok:
			ops = append(ops, w)
		case s == "--":
			done = true
		case strings.HasPrefix(s, "-") && len(s) > 1:
		default:
			ops = append(ops, w)
		}
	}
	if needsSeparator(ops) {
		return append([]*syntax.Word{word("--")}, ops...)
	}
	return ops
}

func needsSeparator(ops []*syntax.Word) bool {
	for _, w := range ops {
		if s, ok := literal(w); ok && strings.HasPrefix(s, "-") && s != "-" {
			return true
		}
	}
	return false
}

func rewriteRM(c *syntax.CallExpr) {
	e, ok := effectiveCommand(c)
	if !ok {
		return
	}
	switch e.name {
	case "rm":
		c.Args = append(append(append([]*syntax.Word{}, c.Args[:e.index]...),
			word("recycler"), word("trash")), rmTargets(c.Args[e.index+1:])...)
	case "xargs":
		// Here `rm` is an ARGUMENT word of xargs, not a command word, so the
		// effective-command resolver never sees it.
		u, found := xargsUtility(c.Args, e.index+1)
		if !found {
			return
		}
		if s, _ := literal(c.Args[u]); s != "rm" {
			return
		}
		c.Args = append(append(append([]*syntax.Word{}, c.Args[:u]...),
			word("recycler"), word("trash")), rmTargets(c.Args[u+1:])...)
	}
}

var xargsValuedFlag = regexp.MustCompile(`^-[nPIisLdEa]$`)

// xargsUtility finds the word naming the utility xargs runs. An xargs flag
// that takes a separated value must not be mistaken for it.
func xargsUtility(args []*syntax.Word, start int) (int, bool) {
	skip := false
	for i := start; i < len(args); i++ {
		if skip {
			skip = false
			continue
		}
		s, ok := literal(args[i])
		switch {
		case !ok:
			return 0, false
		case xargsValuedFlag.MatchString(s):
			skip = true
		case strings.HasPrefix(s, "-") && len(s) > 1:
		default:
			return i, true
		}
	}
	return 0, false
}

func rewriteTruncate(c *syntax.CallExpr) {
	e, ok := effectiveCommand(c)
	if !ok || e.name != "truncate" || !isTruncateZero(c.Args[e.index+1:]) {
		return
	}
	targets, ok := truncateTargets(c.Args[e.index+1:])
	if !ok {
		return
	}
	c.Args = append(append(append([]*syntax.Word{}, c.Args[:e.index]...),
		word("recycler"), word("trash")), targets...)
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
