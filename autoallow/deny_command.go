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
			if w.Until {
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
// subshells and conditionals the allow path refuses to read -- a denied program
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
			if !shellwalk.MatchesProgram(name, d.Name) {
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
