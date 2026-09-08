// trailing.go holds the rules anchored at the END of the command.
//
// A limiting pipe, a stdout redirect or a swallowed exit status mid-script is a
// deliberate part of a longer script and is preserved. The same shape on the
// last top-level statement is what truncates the answer the reader was about to
// get, so it is stripped there and nowhere else.
//
// The anchor is the last top-level statement, descending the right branch of
// its && / || chain, which is a leaf under left association.
package bashclean

import (
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func trailing(f *syntax.File, fn func(*syntax.Stmt)) {
	if len(f.Stmts) > 0 {
		fn(f.Stmts[len(f.Stmts)-1])
	}
}

// spineLeaf is the trailing leaf of a statement: && / || chains parse
// left-associative, so the rightmost leaf is the right branch.
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

// stripOrTrue restores the real exit status.
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
// the output lands in the file AND stays visible. A statement with several
// stdout file redirects, a /dev/ target, or a process-substitution target is
// left alone.
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

// ensurePipefail keeps the producer's exit status observable through the pipe
// the tee rewrite introduces. A strictness setting the caller wrote is never
// removed.
func ensurePipefail(f *syntax.File) {
	if len(f.Stmts) == 0 || enablesPipefail(leftmost(f.Stmts[0])) {
		return
	}
	f.Stmts = append([]*syntax.Stmt{{Cmd: &syntax.CallExpr{
		Args: []*syntax.Word{word("set"), word("-o"), word("pipefail")}}}}, f.Stmts...)
}
