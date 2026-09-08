package bashclean

import (
	"fmt"
	"io"
	"os"
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

// Transform applies the rules to command. A parse failure fails open.
func Transform(command string) Result { return transform(command, maxPasses, os.Stderr) }

// transform takes the pass bound and the warning sink as arguments, so a test
// drives the exhaustion path without a global a parallel sibling can see.
func transform(command string, passes int, warn io.Writer) Result {
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
	if divertsGoToolchain(f) {
		return deny(command, "toolchain_output")
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
	// Runs to a fixed point: a rule's output is another rule's input, and a
	// single pass leaves the later rewrite undone.
	if !runToFixedPoint(f, apply, passes) {
		reportNonConvergence(warn, command, printFile(f), passes)
	}
	apply("pipefail", ensurePipefail)
	after := printFile(f)
	return Result{Command: after, Changed: before != after, Rules: rules}
}

// maxPasses bounds the fixed-point loop. Raising it answers no non-convergence.
const maxPasses = 20

// runToFixedPoint applies the rules until the printed tree stops changing, and
// reports false when the bound ran out.
func runToFixedPoint(f *syntax.File, apply func(string, func(*syntax.File)), passes int) bool {
	for range passes {
		pass := printFile(f)
		onePass(apply)
		if printFile(f) == pass {
			return true
		}
	}
	return false
}

// reportNonConvergence says what the bound dropped. The rewrite it emits is
// half-applied, so it can be worse than either endpoint. It goes to stderr
// rather than the debug log, because it reports a defect in the rules and must
// be visible without a log path set.
func reportNonConvergence(warn io.Writer, original, partial string, passes int) {
	fmt.Fprintf(warn,
		"cleanup-bash-cmds: the rewrite did not reach a fixed point in %d passes, "+
			"so a pair of rules is undoing each other. Emitting the partial rewrite.\n"+
			"  original: %q\n  partial:  %q\n",
		passes, original, partial)
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
func literal(w *syntax.Word) (string, bool) { return wordLiteral(w) }
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

// anyCall asks p of every CallExpr in the tree, command substitutions
// included.
func anyCall(f *syntax.File, p func(*syntax.CallExpr) bool) bool {
	hit := false
	walkCalls(f, func(c *syntax.CallExpr) {
		if p(c) {
			hit = true
		}
	})
	return hit
}

func dockerCompose(c *syntax.CallExpr) {
	if len(c.Args) >= 3 && isWord(c.Args[0], "docker") && isWord(c.Args[1], "compose") && isWord(c.Args[2], "restart") {
		c.Args = append([]*syntax.Word{word("docker"), word("compose"), word("up"), word("-d"), word("--force-recreate")}, c.Args[3:]...)
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
			c.Args = []*syntax.Word{c.Args[0], word("3")}
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

// rmTargets returns the real targets of an `rm` call. `--` ends flag parsing
// and is RE-EMITTED in front of a dash-leading name, so the filename is not
// re-read as a recycler flag.
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
	if !ok || e.name != "rm" {
		return
	}
	c.Args = append(append(append([]*syntax.Word{}, c.Args[:e.index]...),
		word("recycler"), word("trash")), rmTargets(c.Args[e.index+1:])...)
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
