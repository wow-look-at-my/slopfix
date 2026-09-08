package bashclean

import (
	"regexp"
	"strings"

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

var lookupFlag = regexp.MustCompile(`[vV]`)

func effectiveCommand(c *syntax.CallExpr) (effCmd, bool) {
	if c == nil {
		return effCmd{}, false
	}
	return resolveCommand(c.Args, 0, 10)
}
func resolveCommand(a []*syntax.Word, i, d int) (effCmd, bool) {
	if d <= 0 || i >= len(a) {
		return effCmd{}, false
	}
	s, ok := wordLiteral(a[i])
	if !ok {
		return effCmd{}, false
	}
	s = strings.TrimPrefix(s, `\`)
	if s != "command" && s != "builtin" {
		return effCmd{s, i}, true
	}
	for j := i + 1; j < len(a); j++ {
		f, ok := wordLiteral(a[j])
		if !ok {
			return effCmd{}, false
		}
		if f == "--" {
			return resolveCommand(a, j+1, d-1)
		}
		if strings.HasPrefix(f, "-") && len(f) > 1 {
			if s == "command" && lookupFlag.MatchString(f) {
				return effCmd{}, false
			}
			continue
		}
		return resolveCommand(a, j, d-1)
	}
	return effCmd{}, false
}
