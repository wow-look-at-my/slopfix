package bashclean

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/shellwalk"
	"mvdan.cc/sh/v3/syntax"
)

func wordLiteral(w *syntax.Word) (string, bool) {
	if w == nil || len(w.Parts) == 0 {
		return "", false
	}
	var b strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			b.WriteString(x.Value)
		case *syntax.SglQuoted:
			b.WriteString(x.Value)
		case *syntax.DblQuoted:
			for _, q := range x.Parts {
				l, ok := q.(*syntax.Lit)
				if !ok {
					return "", false
				}
				b.WriteString(l.Value)
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

type effCmd struct {
	name  string
	index int
}

// effectiveCommand names the program a call really runs, and where its word
// sits in .Args. shellwalk peels the wrappers, so the `rm` rule covers a
// `sudo`, `env`, `nice`, `timeout` or `xargs` prefix and an absolute path
// alike. A non-static command word and a lookup both resolve to nothing.
func effectiveCommand(c *syntax.CallExpr) (effCmd, bool) {
	if c == nil || len(c.Args) == 0 {
		return effCmd{}, false
	}
	argv := shellwalk.Words(c.Args)
	eff := shellwalk.StripWrappers(argv)
	if len(eff) == 0 || !eff[0].Static || eff[0].Text == "" {
		return effCmd{}, false
	}
	return effCmd{shellwalk.CommandName(eff[0].Text), len(argv) - len(eff)}, true
}
