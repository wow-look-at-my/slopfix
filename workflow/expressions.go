// expressions.go turns each ${{ }} expression of a moved script into an
// argument the script reads.
package workflow

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// expression matches a ${{ }} expression. GitHub writes its value into the script text before the shell reads it.
var expression = regexp.MustCompile(`(?s)\$\{\{(.*?)\}\}`)

// quoting is what surrounds an expression in the script.
type quoting int

const (
	bare quoting = iota
	doubleQuoted
	singleQuoted
	dollarQuoted
	arithmetic
	expandedHeredoc
	literalHeredoc
)

// region is a stretch of the script under one quoting, or a function body.
type region struct {
	from, to int
	quoting  quoting
}

// body answers the script as the file holds it, and the expressions it reads
// in argument order. Each distinct expression becomes an argument, and its
// text in the script reads that argument where the expression stood. It
// reports false when the shell cannot parse the script, or an expression sits
// in a heredoc that expands nothing.
func (s shellStep) body(offset int) (string, []string, bool) {
	script := s.run.Value
	if !strings.HasSuffix(script, "\n") {
		script += "\n"
	}
	found := expression.FindAllStringSubmatchIndex(script, -1)
	if len(found) == 0 {
		return script, nil, true
	}
	var args []string
	index := map[string]int{}
	var marked strings.Builder
	marks := make([]int, len(found))
	which := make([]int, len(found))
	at := 0
	for i, m := range found {
		inner := strings.TrimSpace(script[m[2]:m[3]])
		if _, seen := index[inner]; !seen {
			args = append(args, inner)
			index[inner] = len(args)
		}
		which[i] = index[inner]
		marked.WriteString(script[at:m[0]])
		marks[i] = marked.Len()
		marked.WriteString(placeholder(i))
		at = m[1]
	}
	marked.WriteString(script[at:])
	lang := syntax.LangBash
	if strings.HasPrefix(s.shell, "sh") {
		lang = syntax.LangPOSIX
	}
	parsed, err := syntax.NewParser(syntax.KeepComments(true), syntax.Variant(lang)).Parse(strings.NewReader(marked.String()), "")
	if err != nil {
		return "", nil, false
	}
	quotes, functions, positional := regions(parsed)
	capture := positional
	kinds := make([]quoting, len(found))
	for i, from := range marks {
		kinds[i] = innermost(quotes, from)
		if kinds[i] == literalHeredoc {
			return "", nil, false
		}
		capture = capture || inside(functions, from)
	}
	text := marked.String()
	var out strings.Builder
	if capture {
		// A function reads its own arguments, and the script may move its own, so each value is held in a variable first.
		for k := range args {
			fmt.Fprintf(&out, "slopfix_arg_%d=%s\n", k+1, quoted(bare, positionalName(offset+k+1)))
		}
		out.WriteString("set --\n")
	}
	at = 0
	for i, from := range marks {
		out.WriteString(text[at:from])
		name := positionalName(offset + which[i])
		if capture {
			name = "{slopfix_arg_" + strconv.Itoa(which[i]) + "}"
		}
		out.WriteString(quoted(kinds[i], name))
		at = from + len(placeholder(i))
	}
	out.WriteString(text[at:])
	return out.String(), args, true
}

// placeholder is the word that stands where expression i stood while the shell parser reads the script.
func placeholder(i int) string { return "slopfix_expression_" + strconv.Itoa(i) + "_" }

// positionalName answers what follows the $ that reads argument n.
func positionalName(n int) string {
	if n < 10 {
		return strconv.Itoa(n)
	}
	return "{" + strconv.Itoa(n) + "}"
}

// quoted answers the text that reads $name where the quoting q holds.
func quoted(q quoting, name string) string {
	switch q {
	case bare:
		return `"$` + name + `"`
	case singleQuoted:
		return `'"$` + name + `"'`
	case dollarQuoted:
		return `'"$` + name + `"$'`
	}
	return "$" + name
}

// regions answers the quoted stretches of a script, its function bodies, and
// whether its own top level reads or moves its arguments.
func regions(parsed *syntax.File) (quotes, functions []region, positional bool) {
	var reads []int
	span := func(n syntax.Node, q quoting) region {
		return region{from: int(n.Pos().Offset()), to: int(n.End().Offset()), quoting: q}
	}
	syntax.Walk(parsed, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.SglQuoted:
			if n.Dollar {
				quotes = append(quotes, span(n, dollarQuoted))
			} else {
				quotes = append(quotes, span(n, singleQuoted))
			}
		case *syntax.DblQuoted:
			quotes = append(quotes, span(n, doubleQuoted))
		case *syntax.CmdSubst:
			quotes = append(quotes, span(n, bare))
		case *syntax.ProcSubst:
			quotes = append(quotes, span(n, bare))
		case *syntax.ArithmExp:
			quotes = append(quotes, span(n, arithmetic))
		case *syntax.ArithmCmd:
			quotes = append(quotes, span(n, arithmetic))
		case *syntax.Redirect:
			if n.Hdoc != nil {
				q := expandedHeredoc
				if literalDelimiter(n.Word) {
					q = literalHeredoc
				}
				quotes = append(quotes, span(n.Hdoc, q))
			}
		case *syntax.FuncDecl:
			if n.Body != nil {
				functions = append(functions, span(n.Body, bare))
			}
		case *syntax.ParamExp:
			if n.Param != nil && argumentParam(n.Param.Value) {
				reads = append(reads, int(n.Pos().Offset()))
			}
		case *syntax.CallExpr:
			if movesArguments(n) {
				reads = append(reads, int(n.Pos().Offset()))
			}
		}
		return true
	})
	for _, at := range reads {
		if !inside(functions, at) {
			positional = true
		}
	}
	return quotes, functions, positional
}

// argumentParam reports a parameter that reads the script's arguments.
func argumentParam(name string) bool {
	if name == "@" || name == "*" || name == "#" {
		return true
	}
	n, err := strconv.Atoi(name)
	return err == nil && n > 0
}

// movesArguments reports a command that reads or replaces the script's arguments.
func movesArguments(call *syntax.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	switch call.Args[0].Lit() {
	case "shift", "getopts":
		return true
	case "set":
		for _, arg := range call.Args[1:] {
			lit := arg.Lit()
			if lit == "--" || (lit != "" && lit[0] != '-' && lit[0] != '+') {
				return true
			}
		}
	}
	return false
}

// literalDelimiter reports a heredoc delimiter written with a quote, which
// makes the heredoc expand nothing.
func literalDelimiter(word *syntax.Word) bool {
	if word == nil {
		return false
	}
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.SglQuoted, *syntax.DblQuoted:
			return true
		case *syntax.Lit:
			if strings.Contains(p.Value, `\`) {
				return true
			}
		}
	}
	return false
}

// innermost answers the quoting of the narrowest region that holds a byte.
func innermost(quotes []region, at int) quoting {
	best, found := region{}, false
	for _, r := range quotes {
		if at < r.from || at >= r.to {
			continue
		}
		if !found || r.from > best.from || (r.from == best.from && r.to < best.to) {
			best, found = r, true
		}
	}
	return best.quoting
}

// inside reports whether a byte sits in any of the regions.
func inside(regions []region, at int) bool {
	for _, r := range regions {
		if at >= r.from && at < r.to {
			return true
		}
	}
	return false
}
