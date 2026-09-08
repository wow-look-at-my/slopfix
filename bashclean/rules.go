package bashclean

import (
	"mvdan.cc/sh/v3/syntax"
	"regexp"
	"strings"
)

var (
	magicPath      = regexp.MustCompile(`^/(proc|sys|dev)(/|$)`)
	longValuedRead = regexp.MustCompile(`^--(lines|bytes|sleep-interval|pid|max-unchanged-stats)$`)
	shortValued    = regexp.MustCompile(`^-[A-Za-z]*[ncs]$`)
	oldStyleLimit  = regexp.MustCompile(`^\+[0-9]+$`)
	sedQuietFlag   = regexp.MustCompile(`^-[A-Za-z]*n[A-Za-z]*$`)
	sedScriptFlag  = regexp.MustCompile(`^-[A-Za-z]*[ef][A-Za-z]*$`)
)

// readOperands returns the file operands of a cat/head/tail call. Flags are
// skipped, and so is the separated VALUE of a value-taking flag (-n 5, -c 10,
// --lines=, +N), so `head -n 20 f` does not mistake `20` for a file. A lone
// `-` is stdin, not a file. valued enables head/tail's flag grammar; cat has
// no value-taking flag, so for it every dash word is simply dropped.
func readOperands(args []*syntax.Word, valued bool) []*syntax.Word {
	ops := []*syntax.Word{}
	skip, done := false, false
	for _, w := range args {
		if skip {
			skip = false
			continue
		}
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
		case s == "-":
		case strings.HasPrefix(s, "--"):
			if valued && longValuedRead.MatchString(s) {
				skip = true
			}
		case strings.HasPrefix(s, "-") && len(s) > 1:
			if valued && shortValued.MatchString(s) {
				skip = true
			}
		case valued && oldStyleLimit.MatchString(s):
		default:
			ops = append(ops, w)
		}
	}
	return ops
}

func namesRealFile(ops []*syntax.Word) bool {
	for _, w := range ops {
		if s, ok := literal(w); ok && !magicPath.MatchString(s) {
			return true
		}
	}
	return false
}

// sedSuppressesOutput: a sed that suppresses automatic printing is SELECTING
// lines to print, which is a file read when it names a file. A sed without it
// transforms its input and is left alone.
func sedSuppressesOutput(args []*syntax.Word) bool {
	for _, w := range args {
		s, ok := literal(w)
		if ok && (sedQuietFlag.MatchString(s) || s == "--quiet" || s == "--silent") {
			return true
		}
	}
	return false
}

// sedFileOperands drops the flags and the leading script word, unless -e/-f
// already supplied the script, in which case the first non-flag word is
// already a file.
func sedFileOperands(args []*syntax.Word) []*syntax.Word {
	scripted := false
	for _, w := range args {
		s, ok := literal(w)
		if ok && (sedScriptFlag.MatchString(s) ||
			strings.HasPrefix(s, "--expression") || strings.HasPrefix(s, "--file")) {
			scripted = true
		}
	}
	ops := []*syntax.Word{}
	for _, w := range args {
		if s, ok := literal(w); ok && strings.HasPrefix(s, "-") {
			continue
		}
		ops = append(ops, w)
	}
	if !scripted && len(ops) > 0 {
		ops = ops[1:]
	}
	return ops
}

func callIsSedSuppressing(c *syntax.CallExpr) bool {
	e, ok := effectiveCommand(c)
	return ok && e.name == "sed" && sedSuppressesOutput(c.Args[e.index+1:])
}

func callIsSedLineRead(c *syntax.CallExpr) bool {
	if !callIsSedSuppressing(c) {
		return false
	}
	e, _ := effectiveCommand(c)
	return namesRealFile(sedFileOperands(c.Args[e.index+1:]))
}

func callReadsBannedFile(c *syntax.CallExpr) bool {
	e, ok := effectiveCommand(c)
	if !ok || (e.name != "cat" && e.name != "head" && e.name != "tail") {
		return false
	}
	return namesRealFile(readOperands(c.Args[e.index+1:], e.name != "cat"))
}

func hasFileRead(f *syntax.File) bool {
	return hasStatementCall(f, func(c *syntax.CallExpr) bool {
		return callReadsBannedFile(c) || callIsSedLineRead(c)
	})
}
func hasGitRM(f *syntax.File) bool {
	return hasStatementCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		if !ok || e.name != "git" {
			return false
		}
		for _, w := range c.Args[e.index+1:] {
			s, st := literal(w)
			if st && s == "--cached" {
				return false
			}
			if st && !strings.HasPrefix(s, "-") {
				return s == "rm"
			}
		}
		return false
	})
}
var (
	sizeZeroFlag = regexp.MustCompile(`^(-s|--size=)0+$`)
	allZeros     = regexp.MustCompile(`^0+$`)
	droppableRM  = regexp.MustCompile(`^--(recursive|force|verbose|interactive)$|^-[rRfvIi]+$`)
)

// isTruncateZero: `truncate -s 0 f`, `--size=0`, `-s0` all empty a file.
func isTruncateZero(args []*syntax.Word) bool {
	for i, w := range args {
		s, ok := literal(w)
		if !ok {
			continue
		}
		if sizeZeroFlag.MatchString(s) {
			return true
		}
		if (s == "-s" || s == "--size") && i+1 < len(args) {
			if n, ok := literal(args[i+1]); ok && allZeros.MatchString(n) {
				return true
			}
		}
	}
	return false
}

// truncateTargets returns the files a zero-size truncate would empty. Any
// OTHER flag fails, which sends the call to the deny: `-r RFILE` names a
// reference file rather than a target, so dropping the flag and keeping its
// value would recycle a file the command never touched.
func truncateTargets(args []*syntax.Word) ([]*syntax.Word, bool) {
	ops := []*syntax.Word{}
	skip, done := false, false
	for _, w := range args {
		if done {
			ops = append(ops, w)
			continue
		}
		if skip {
			skip = false
			continue
		}
		s, ok := literal(w)
		switch {
		case !ok:
			ops = append(ops, w)
		case s == "--":
			done = true
		case sizeZeroFlag.MatchString(s):
		case s == "-s" || s == "--size":
			skip = true
		case strings.HasPrefix(s, "-") && len(s) > 1:
			return nil, false
		default:
			ops = append(ops, w)
		}
	}
	if len(ops) == 0 {
		return nil, false
	}
	if needsSeparator(ops) {
		return append([]*syntax.Word{word("--")}, ops...), true
	}
	return ops, true
}

func hasBadTruncate(f *syntax.File) bool {
	return hasStatementCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		if !ok || e.name != "truncate" || !isTruncateZero(c.Args[e.index+1:]) {
			return false
		}
		_, fine := truncateTargets(c.Args[e.index+1:])
		return !fine
	})
}

// rmUnknownFlags collects the flags this rule refuses to translate. `--` ends
// flag parsing, so everything after it is a path however dash-leading, and a
// lone `-` is a filename.
func rmUnknownFlags(args []*syntax.Word) bool {
	for _, w := range args {
		s, ok := literal(w)
		if !ok {
			continue
		}
		if s == "--" {
			return false
		}
		if strings.HasPrefix(s, "-") && len(s) > 1 && !droppableRM.MatchString(s) {
			return true
		}
	}
	return false
}

func hasBadRM(f *syntax.File) bool {
	return hasStatementCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		if !ok {
			return false
		}
		switch e.name {
		case "rm":
			return rmUnknownFlags(c.Args[e.index+1:])
		case "xargs":
			u, found := xargsUtility(c.Args, e.index+1)
			if !found {
				return false
			}
			if s, _ := literal(c.Args[u]); s != "rm" {
				return false
			}
			return rmUnknownFlags(c.Args[u+1:])
		}
		return false
	})
}

func trailing(f *syntax.File, fn func(*syntax.Stmt)) {
	if len(f.Stmts) > 0 {
		fn(f.Stmts[len(f.Stmts)-1])
	}
}
// spineLeaf is the string-end leaf of a statement: && / || chains parse
// left-associative, so the rightmost leaf is one level down.
func spineLeaf(s *syntax.Stmt) *syntax.Stmt {
	if b, ok := s.Cmd.(*syntax.BinaryCmd); ok && (b.Op == syntax.AndStmt || b.Op == syntax.OrStmt) {
		return b.Y
	}
	return s
}

// lastStage is the textual end of a pipeline.
func lastStage(s *syntax.Stmt) *syntax.Stmt {
	if b, ok := s.Cmd.(*syntax.BinaryCmd); ok && (b.Op == syntax.Pipe || b.Op == syntax.PipeAll) {
		return b.Y
	}
	return s
}

// promoteInto lifts inner over outer, keeping outer's statement flags and
// appending outer's redirects after inner's own.
func promoteInto(outer, inner *syntax.Stmt) {
	redirs := append(append([]*syntax.Redirect{}, inner.Redirs...), outer.Redirs...)
	outer.Negated = outer.Negated || inner.Negated
	outer.Background = outer.Background || inner.Background
	outer.Coprocess = outer.Coprocess || inner.Coprocess
	outer.Cmd = inner.Cmd
	outer.Redirs = redirs
}

func isHeadTailStage(c *syntax.CallExpr) bool {
	n := cmd(c)
	return n == "head" || n == "tail" || callIsSedSuppressing(c)
}

func isGrepStage(c *syntax.CallExpr) bool { return cmd(c) == "grep" }

// stripStages removes trailing pipeline stages matching pred, repeatedly.
func stripStages(s *syntax.Stmt, pred func(*syntax.CallExpr) bool) {
	for {
		b, ok := s.Cmd.(*syntax.BinaryCmd)
		if !ok || b.Op != syntax.Pipe {
			return
		}
		c, ok := b.Y.Cmd.(*syntax.CallExpr)
		if !ok || len(c.Args) == 0 || !pred(c) {
			return
		}
		promoteInto(s, b.X)
	}
}

func isBareTrue(s *syntax.Stmt) bool {
	c, ok := s.Cmd.(*syntax.CallExpr)
	return ok && len(c.Args) == 1 && cmd(c) == "true" && len(c.Assigns) == 0 &&
		len(s.Redirs) == 0 && !s.Negated && !s.Background
}

func stripOrTrue(s *syntax.Stmt) {
	for {
		b, ok := s.Cmd.(*syntax.BinaryCmd)
		if !ok || b.Op != syntax.OrStmt || !isBareTrue(b.Y) {
			return
		}
		promoteInto(s, b.X)
	}
}
func cmdCall(c syntax.Command) string {
	if x, ok := c.(*syntax.CallExpr); ok {
		return cmd(x)
	}
	return ""
}
func stripMerge(s *syntax.Stmt) {
	if len(s.Redirs) > 0 {
		r := s.Redirs[len(s.Redirs)-1]
		if r.Op == syntax.DplOut && r.N != nil && r.N.Value == "2" && isWord(r.Word, "1") {
			s.Redirs = s.Redirs[:len(s.Redirs)-1]
		}
	}
}
func tee(s *syntax.Stmt) {
	if len(s.Redirs) != 1 {
		return
	}
	r := s.Redirs[0]
	if r.Op != syntax.RdrOut && r.Op != syntax.AppOut {
		return
	}
	if strings.HasPrefix(litOf(r.Word), "/dev/") {
		return
	}
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok {
		return
	}
	s.Cmd = &syntax.BinaryCmd{Op: syntax.Pipe, X: &syntax.Stmt{Cmd: c}, Y: &syntax.Stmt{Cmd: &syntax.CallExpr{Args: []*syntax.Word{word("tee")}}}}
	if r.Op == syntax.AppOut {
		s.Cmd.(*syntax.BinaryCmd).Y.Cmd.(*syntax.CallExpr).Args = append(s.Cmd.(*syntax.BinaryCmd).Y.Cmd.(*syntax.CallExpr).Args, word("-a"))
	}
	s.Cmd.(*syntax.BinaryCmd).Y.Cmd.(*syntax.CallExpr).Args = append(s.Cmd.(*syntax.BinaryCmd).Y.Cmd.(*syntax.CallExpr).Args, r.Word)
	s.Redirs = nil
}
func ensurePipefail(f *syntax.File) {
	if len(f.Stmts) == 0 || cmdCall(f.Stmts[0].Cmd) == "set" {
		return
	}
	f.Stmts = append([]*syntax.Stmt{{Cmd: &syntax.CallExpr{Args: []*syntax.Word{word("set"), word("-o"), word("pipefail")}}}}, f.Stmts...)
}
func narration(f *syntax.File) {
	walkCalls(f, func(c *syntax.CallExpr) {
		e, ok := effectiveCommand(c)
		if !ok || (e.name != "echo" && e.name != "printf") || len(c.Args) <= e.index+1 {
			return
		}
		if allStatic(c.Args[e.index+1:]) {
			c.Args = []*syntax.Word{word(":")}
		}
	})
}
