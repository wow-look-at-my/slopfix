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
// skipped, and so is the separated VALUE of a value-taking flag, so a count
// is never mistaken for a file. A lone `-` is stdin. valued enables
// head/tail's flag grammar; for cat every dash word is simply dropped.
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
// already supplied the script, leaving every non-flag word a file.
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
// hasGitRM: the SUBCOMMAND is the leading non-flag word after `git`, so
// `git -C dir rm f` counts while `git commit -m rm` does not. `--cached`
// anywhere only unstages, and passes through.
func hasGitRM(f *syntax.File) bool {
	return hasStatementCall(f, func(c *syntax.CallExpr) bool {
		e, ok := effectiveCommand(c)
		if !ok || e.name != "git" {
			return false
		}
		sub, skip := "", false
		for _, w := range c.Args[e.index+1:] {
			s, st := literal(w)
			if st && s == "--cached" {
				return false
			}
			if sub != "" || skip {
				skip = false
				continue
			}
			switch {
			case !st:
				sub = "?"
			case s == "-C" || s == "-c":
				skip = true
			case strings.HasPrefix(s, "-"):
			default:
				sub = s
			}
		}
		return sub == "rm"
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

// hasBadRM scans the whole tree, the same reach as the rewrite's walk, so an
// `rm` inside `$( )` is covered too.
func hasBadRM(f *syntax.File) bool {
	return anyCall(f, func(c *syntax.CallExpr) bool {
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
func stripMerge(s *syntax.Stmt) {
	for len(s.Redirs) > 0 {
		r := s.Redirs[len(s.Redirs)-1]
		if r.Op != syntax.DplOut || r.N == nil || r.N.Value != "2" || !isWord(r.Word, "1") {
			return
		}
		s.Redirs = s.Redirs[:len(s.Redirs)-1]
	}
}

func isStdoutFileAny(r *syntax.Redirect) bool {
	return (r.N == nil || r.N.Value == "1") && (r.Op == syntax.RdrOut || r.Op == syntax.AppOut)
}

// leadingLit is a word's leading literal text, "" when it starts with an
// expansion.
func leadingLit(w *syntax.Word) string {
	if w == nil || len(w.Parts) == 0 {
		return ""
	}
	switch p := w.Parts[0].(type) {
	case *syntax.Lit:
		return p.Value
	case *syntax.SglQuoted:
		return p.Value
	case *syntax.DblQuoted:
		if len(p.Parts) > 0 {
			if l, ok := p.Parts[0].(*syntax.Lit); ok {
				return l.Value
			}
		}
	}
	return ""
}

func isStdoutFileTeeable(r *syntax.Redirect) bool {
	if !isStdoutFileAny(r) || r.Word == nil || len(r.Word.Parts) == 0 {
		return false
	}
	if _, ok := r.Word.Parts[0].(*syntax.ProcSubst); ok {
		return false
	}
	return !strings.HasPrefix(leadingLit(r.Word), "/dev/")
}

// teeRewrite turns a trailing stdout file redirect into a pipe through tee, so
// the output lands in the file AND stays visible. A statement with more than
// one stdout file redirect, a /dev/ target, or a process-substitution target
// is left alone.
func teeRewrite(s *syntax.Stmt) {
	redirs := &s.Redirs
	if b, ok := s.Cmd.(*syntax.BinaryCmd); ok && b.Op == syntax.Pipe {
		redirs = &b.Y.Redirs
	}
	var found []*syntax.Redirect
	for _, r := range *redirs {
		if isStdoutFileAny(r) {
			found = append(found, r)
		}
	}
	if len(found) != 1 || !isStdoutFileTeeable(found[0]) {
		return
	}
	r := found[0]
	kept := []*syntax.Redirect{}
	for _, x := range *redirs {
		if !isStdoutFileAny(x) {
			kept = append(kept, x)
		}
	}
	*redirs = kept
	producer := &syntax.Stmt{Cmd: s.Cmd, Redirs: s.Redirs}
	teeArgs := []*syntax.Word{word("tee")}
	if r.Op == syntax.AppOut {
		teeArgs = append(teeArgs, word("-a"))
	}
	teeArgs = append(teeArgs, r.Word)
	s.Cmd = &syntax.BinaryCmd{
		Op: syntax.Pipe,
		X:  producer,
		Y:  &syntax.Stmt{Cmd: &syntax.CallExpr{Args: teeArgs}},
	}
	s.Redirs = nil
}

var setOFlag = regexp.MustCompile(`^-[A-Za-z]*o$`)

// enablesPipefail: `set -o pipefail`, `set -eo pipefail`, `set -e -o pipefail`
// and multiple -o pairs all count.
func enablesPipefail(s *syntax.Stmt) bool {
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok || cmd(c) != "set" {
		return false
	}
	words := make([]string, 0, len(c.Args))
	for _, w := range c.Args[1:] {
		if v, ok := literal(w); ok {
			words = append(words, v)
		} else {
			words = append(words, " ")
		}
	}
	for i := 0; i+1 < len(words); i++ {
		if setOFlag.MatchString(words[i]) && words[i+1] == "pipefail" {
			return true
		}
	}
	return false
}

func leftmost(s *syntax.Stmt) *syntax.Stmt {
	for {
		b, ok := s.Cmd.(*syntax.BinaryCmd)
		if !ok || b.Op != syntax.AndStmt {
			return s
		}
		s = b.X
	}
}

func ensurePipefail(f *syntax.File) {
	if len(f.Stmts) == 0 || enablesPipefail(leftmost(f.Stmts[0])) {
		return
	}
	f.Stmts = append([]*syntax.Stmt{{Cmd: &syntax.CallExpr{
		Args: []*syntax.Word{word("set"), word("-o"), word("pipefail")}}}}, f.Stmts...)
}

// An echo becomes the no-op `:` only when its stdout REACHES THE TERMINAL.
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
