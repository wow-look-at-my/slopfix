package bashclean

import (
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// lit builds a bare literal word.
func lit(v string) *syntax.Word {
	return &syntax.Word{Parts: []syntax.WordPart{&syntax.Lit{Value: v}}}
}

// wordLiteral returns a word's static text. ok is false when any part carries
// an expansion, so a caller can never act on text it cannot know.
func wordLiteral(w *syntax.Word) (string, bool) {
	if w == nil || len(w.Parts) == 0 {
		return "", false
	}
	var b strings.Builder
	for _, p := range w.Parts {
		switch t := p.(type) {
		case *syntax.Lit:
			b.WriteString(t.Value)
		case *syntax.SglQuoted:
			b.WriteString(t.Value)
		case *syntax.DblQuoted:
			for _, ip := range t.Parts {
				l, ok := ip.(*syntax.Lit)
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

// litOf is wordLiteral with the failure folded into an empty string.
func litOf(w *syntax.Word) string {
	s, ok := wordLiteral(w)
	if !ok {
		return ""
	}
	return s
}

// wordIs reports whether w is exactly the literal v, in any quoting.
func wordIs(w *syntax.Word, v string) bool {
	if w == nil || len(w.Parts) != 1 {
		return false
	}
	switch t := w.Parts[0].(type) {
	case *syntax.Lit:
		return t.Value == v
	case *syntax.SglQuoted:
		return t.Value == v
	case *syntax.DblQuoted:
		if len(t.Parts) != 1 {
			return false
		}
		l, ok := t.Parts[0].(*syntax.Lit)
		return ok && l.Value == v
	}
	return false
}

// wordLitPrefix is the leading literal text of a word, empty when the word
// opens with an expansion.
func wordLitPrefix(w *syntax.Word) string {
	if w == nil || len(w.Parts) == 0 {
		return ""
	}
	switch t := w.Parts[0].(type) {
	case *syntax.Lit:
		return t.Value
	case *syntax.SglQuoted:
		return t.Value
	case *syntax.DblQuoted:
		if len(t.Parts) > 0 {
			if l, ok := t.Parts[0].(*syntax.Lit); ok {
				return l.Value
			}
		}
	}
	return ""
}

// callName is the bare command word of a plain call, empty for anything else.
func callName(cmd syntax.Command) string {
	c, ok := cmd.(*syntax.CallExpr)
	if !ok || len(c.Args) == 0 || len(c.Args[0].Parts) != 1 {
		return ""
	}
	l, ok := c.Args[0].Parts[0].(*syntax.Lit)
	if !ok {
		return ""
	}
	return l.Value
}

// effCmd names the command that actually runs, and where its word sits in the
// original argument list.
type effCmd struct {
	name  string
	index int
}

var lookupFlag = regexp.MustCompile(`[vV]`)

// effectiveCommand resolves a quoted or split command word, a leading
// backslash, and the command/builtin wrappers. It answers false for a command
// word carrying an expansion, and for a lookup such as `command -v NAME`.
func effectiveCommand(c *syntax.CallExpr) (effCmd, bool) {
	if c == nil {
		return effCmd{}, false
	}
	return resolveCommand(c.Args, 0, 10)
}

func resolveCommand(args []*syntax.Word, i, depth int) (effCmd, bool) {
	if depth <= 0 || i >= len(args) {
		return effCmd{}, false
	}
	raw, ok := wordLiteral(args[i])
	if !ok {
		return effCmd{}, false
	}
	w := strings.TrimPrefix(raw, `\`)
	if w != "command" && w != "builtin" {
		return effCmd{name: w, index: i}, true
	}
	isCommand := w == "command"
	next, lookup, ok := scanWrapperFlags(args, i+1, isCommand)
	if !ok || lookup {
		return effCmd{}, false
	}
	return resolveCommand(args, next, depth-1)
}

// scanWrapperFlags walks the flags after a wrapper and reports where its
// target word sits.
func scanWrapperFlags(args []*syntax.Word, j int, isCommand bool) (next int, lookup, ok bool) {
	for ; j < len(args); j++ {
		fl, good := wordLiteral(args[j])
		if !good {
			return 0, false, false
		}
		if fl == "--" {
			return j + 1, false, true
		}
		if strings.HasPrefix(fl, "-") && len(fl) > 1 {
			if isCommand && lookupFlag.MatchString(fl) {
				return 0, true, true
			}
			continue
		}
		return j, false, true
	}
	return 0, false, false
}
