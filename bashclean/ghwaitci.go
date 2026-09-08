// ghwaitci.go spells an Actions read the way the shim accepts.
//
// The shim refuses `gh run`, `gh workflow` and `gh pr checks` at run time, so
// each of those spellings is a guaranteed failure that costs a whole tool call.
// Only the forms carrying the same meaning on both sides are rewritten:
//
//	gh run view <id> --log-failed  ->  gh wait-ci log <id> --failed
//	gh run view <id> --log         ->  gh wait-ci log <id>
//	gh run view <id>               ->  gh wait-ci view <id>
//	gh run watch <id>              ->  gh wait-ci <id>
//	gh run rerun <id>              ->  gh wait-ci rerun <id>
//	gh run list [flags]            ->  gh wait-ci runs [flags]
//	gh pr checks [flags]           ->  gh wait-ci checks [flags]
//
// A form outside that table is left alone rather than guessed at: the shim
// already refuses it with the full mapping, and a wrong guess replaces a
// precise message with a command that asks a different question.
package bashclean

import (
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// runID keeps the rewrite off an expansion, as the rest of the rules do.
var runID = regexp.MustCompile(`^[0-9]+$`)

// onlyFlagsAndValues asks whether no remaining word is a POSITIONAL. A bare
// word right after a dash word is that flag's value, while a `--flag=value`
// carries its own. A positional blocks the rewrite, since it names a subject.
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
