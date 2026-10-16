package autoallow

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/shellwalk"
	"mvdan.cc/sh/v3/syntax"
)

// A rule names the command a statement RUNS, never the argv spelling, which
// cannot be enumerated.
type CommandRule struct {
	Name string
	// The section this rule came from: allow, ask or deny.
	Behavior string
	Message  string
	// Deny a script handed to the interpreter, sparing `node script.js`.
	InlineOnly bool
	// Flags taking a script as their value; a single-dash flag matches inside a cluster too, covering perl's -pe and -lane.
	EvalFlags []string
	// Subcommand spellings of the same thing (deno eval).
	EvalSubcommands []string
	// Deny only a command that runs nothing but programs such rules name.
	WholeOnly bool
}

// matchWholeCommand answers the wholeCommand rules. Every statement must run a
// program one of them names, a `set`, or assignments alone. A substitution is
// a statement of its own, so `true $(make)` does work.
func matchWholeCommand(file *syntax.File, rules []CommandRule) (string, string) {
	var hit *CommandRule
	work := false
	syntax.Walk(file, func(n syntax.Node) bool {
		s, ok := n.(*syntax.Stmt)
		if work || !ok {
			return !work
		}
		switch c := s.Cmd.(type) {
		case nil:
			work = len(s.Redirs) > 0
		case *syntax.CallExpr:
			r, nothing := noopCall(s, c, rules)
			if hit == nil {
				hit = r
			}
			work = r == nil && !nothing
		case *syntax.BinaryCmd, *syntax.Block, *syntax.Subshell, *syntax.IfClause,
			*syntax.WhileClause, *syntax.ForClause, *syntax.CaseClause, *syntax.TimeClause:
		default:
			work = true
		}
		return !work
	})
	if work || hit == nil {
		return "", ""
	}
	return hit.Name, hit.Message
}

// noopCall is the rule a bare call runs. The bool reports a call that runs
// nothing a rule names: assignments alone, or a `set`. A redirect, a negation
// or a background job is work.
func noopCall(s *syntax.Stmt, c *syntax.CallExpr, rules []CommandRule) (*CommandRule, bool) {
	if len(s.Redirs) > 0 || s.Negated || s.Background || s.Coprocess {
		return nil, false
	}
	if len(c.Args) == 0 {
		return nil, true
	}
	name, _ := shellwalk.ResolveProgram(shellwalk.Words(c.Args))
	for i := range rules {
		if shellwalk.MatchesProgram(name, rules[i].Name) {
			return &rules[i], false
		}
	}
	return nil, name == "set"
}

// loopsForever: a `while` whose condition is a bare `true` or `:` waits like
// an `until`.
func loopsForever(w *syntax.WhileClause) bool {
	if w.Until || len(w.Cond) != 1 {
		return false
	}
	c, ok := w.Cond[0].Cmd.(*syntax.CallExpr)
	if !ok || len(c.Args) != 1 {
		return false
	}
	name, _ := shellwalk.ResolveProgram(shellwalk.Words(c.Args))
	return name == "true" || name == ":"
}

// matchKeyword answers the rule naming a shell keyword the command uses.
//
// The resolver below reaches a program. It never reaches `until`, because what
// a loop runs is its body. So a keyword is matched on the parse node, and a
// rule is written the same way whichever kind it names.
func matchKeyword(file *syntax.File, rules []CommandRule) (string, string) {
	used := set.New[string]()
	syntax.Walk(file, func(n syntax.Node) bool {
		if w, ok := n.(*syntax.WhileClause); ok {
			if w.Until || loopsForever(w) {
				used.Add("until")
			} else {
				used.Add("while")
			}
		}
		return true
	})
	for _, rule := range rules {
		if used.Contains(rule.Name) {
			return rule.Name, rule.Message
		}
	}
	return "", ""
}

// isInlineScript reports whether an invocation hands the interpreter a script
// rather than a file. fedByStdin carries what the argument list cannot show.
func isInlineScript(d CommandRule, args []shellwalk.Word, fedByStdin bool) bool {
	for i, a := range args {
		arg := a.Text
		if shellwalk.StdinMarkers.Contains(arg) {
			return true
		}
		for _, f := range d.EvalFlags {
			if arg == f {
				return true
			}
			// A single-dash cluster: perl -pe, -ne, -lane, ruby -ne.
			if len(f) == 2 && f[0] == '-' && strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") &&
				strings.ContainsRune(arg[1:], rune(f[1])) {
				return true
			}
		}
		if i == 0 {
			for _, sub := range d.EvalSubcommands {
				if arg == sub {
					return true
				}
			}
		}
	}
	// A named script makes stdin its input, not the program.
	return fedByStdin && !shellwalk.NamesAScript(args)
}

// matchCommandRule walks EVERY statement, including the substitutions,
// subshells and conditionals the allow path refuses to read. A denied program
// must never be a `$(...)` away from running.
func matchCommandRule(command string, denies []CommandRule) (string, string) {
	if len(denies) == 0 {
		return "", ""
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return "", ""
	}
	if name, msg := matchKeyword(file, denies); name != "" {
		return name, msg
	}
	var whole []CommandRule
	for _, d := range denies {
		if d.WholeOnly {
			whole = append(whole, d)
		}
	}
	if name, msg := matchWholeCommand(file, whole); name != "" {
		return name, msg
	}

	// `echo 'code' | node` smuggles a script past an argument check.
	piped := set.New[*syntax.Stmt]()
	syntax.Walk(file, func(n syntax.Node) bool {
		if b, ok := n.(*syntax.BinaryCmd); ok && (b.Op == syntax.Pipe || b.Op == syntax.PipeAll) {
			piped.Add(b.Y)
		}
		return true
	})

	var hitName, hitMsg string
	syntax.Walk(file, func(n syntax.Node) bool {
		if hitName != "" {
			return false
		}
		stmt, ok := n.(*syntax.Stmt)
		if !ok {
			return true
		}
		call, ok := stmt.Cmd.(*syntax.CallExpr)
		if !ok {
			return true
		}
		name, args := shellwalk.ResolveProgram(shellwalk.Words(call.Args))
		if name == "" {
			return true
		}
		fedByStdin := piped.Contains(stmt)
		for _, r := range stmt.Redirs {
			switch r.Op {
			case syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc, syntax.RdrIn:
				fedByStdin = true
			}
		}
		for _, d := range denies {
			if d.WholeOnly || !shellwalk.MatchesProgram(name, d.Name) {
				continue
			}
			if d.InlineOnly && !isInlineScript(d, args, fedByStdin) {
				continue
			}
			hitName, hitMsg = name, d.Message
			return false
		}
		return true
	})
	return hitName, hitMsg
}
