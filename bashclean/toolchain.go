// toolchain.go keeps go-toolchain's output in the transcript.
//
// A pipe, a redirect or a capture truncates exactly what the run was for: the
// coverage list, the test failures, the build errors. The pipeline and the
// redirect are stripped silently, so the command runs bare and the whole output
// lands where it is read. A substitution cannot be stripped without changing
// what the script computes, so it is denied instead.
package bashclean

import (
	"mvdan.cc/sh/v3/syntax"
)

// undivertToolchain strips the pipeline stages and the stdout redirects that
// take a go-toolchain run's output off the transcript.
func undivertToolchain(f *syntax.File) {
	syntax.Walk(f, func(n syntax.Node) bool {
		s, ok := n.(*syntax.Stmt)
		if !ok || s.Cmd == nil {
			return true
		}
		if leaf := pipeHead(s); leaf != s && isGoToolchain(leaf.Cmd) {
			promoteInto(s, leaf)
		}
		if isGoToolchain(s.Cmd) {
			s.Redirs = keepNonStdout(s.Redirs)
		}
		return true
	})
}

// pipeHead is a pipeline's earliest stage, which is its innermost left branch.
func pipeHead(s *syntax.Stmt) *syntax.Stmt {
	for {
		b, ok := s.Cmd.(*syntax.BinaryCmd)
		if !ok || (b.Op != syntax.Pipe && b.Op != syntax.PipeAll) {
			return s
		}
		s = b.X
	}
}

// keepNonStdout drops the redirects that move stdout.
func keepNonStdout(rs []*syntax.Redirect) []*syntax.Redirect {
	kept := []*syntax.Redirect{}
	for _, r := range rs {
		if !redirectsStdout(r) {
			kept = append(kept, r)
		}
	}
	return kept
}

// redirectsStdout reports a redirect that takes stdout off the terminal. `&>`
// and `&>>` count: they carry stdout as well as stderr.
func redirectsStdout(r *syntax.Redirect) bool {
	switch r.Op {
	case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll:
		return r.N == nil || r.N.Value == "1"
	}
	return false
}

// capturesGoToolchain reports a go-toolchain inside a command or process
// substitution. The output goes into a value the script then uses, so there is
// nothing to strip: removing the substitution changes what the script computes.
func capturesGoToolchain(f *syntax.File) bool {
	found := false
	syntax.Walk(f, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.CmdSubst, *syntax.ProcSubst:
			if callsGoToolchain(n) {
				found = true
			}
		}
		return !found
	})
	return found
}

// callsGoToolchain reports the command anywhere inside a substitution, which
// captures whatever it prints however deeply it is nested.
func callsGoToolchain(n syntax.Node) bool {
	found := false
	syntax.Walk(n, func(inner syntax.Node) bool {
		if s, ok := inner.(*syntax.Stmt); ok && isGoToolchain(s.Cmd) {
			found = true
		}
		return !found
	})
	return found
}

func isGoToolchain(cmd syntax.Command) bool {
	call, ok := cmd.(*syntax.CallExpr)
	if !ok {
		return false
	}
	e, ok := effectiveCommand(call)
	return ok && e.name == "go-toolchain"
}
