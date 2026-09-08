// redirects.go scrubs the stderr discard, tree-wide.
//
// stderr is how a command reports its own failure, so a discarded stderr costs
// another call asking whether it worked. A discard of stderr alone is dropped.
// A discard of both streams is demoted to its stdout-only spelling, which keeps
// the half the command asked for and frees only stderr.
//
// A bare merge of stderr onto stdout survives: it is a merge, not a discard. It
// goes only where an EARLIER entry in the same list already sent stdout to
// /dev/null, so the reversed spelling, which sends stderr to the terminal, is
// left alone. Each predicate below names the shape it matches.
package bashclean

import "mvdan.cc/sh/v3/syntax"

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

// A positional pass over a Redirs list: order decides whether a trailing
// merge lands in /dev/null or on the terminal.
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
