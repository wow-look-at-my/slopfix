// extract.go moves a run: script that holds a test into a file of its own.
//
// A line of a run: script is shell. A cut of the line that asserts can leave
// an if with an empty then, so the repair cuts no line. The whole script moves
// into a file, and the step runs the file.
package workflow

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	yaml "go.yaml.in/yaml/v3"
	"mvdan.cc/sh/v3/syntax"
)

// ScriptDir is where a moved script lives, from the checkout root.
const ScriptDir = ".github/scripts"

// scriptNames bounds the suffixes tried for a script name another file holds.
const scriptNames = 100

// expression matches a ${{ }} expression. GitHub writes its value into the script text before the shell reads it.
var expression = regexp.MustCompile(`(?s)\$\{\{(.*?)\}\}`)

// trailingComment matches a YAML comment at the end of a row.
var trailingComment = regexp.MustCompile(`[ \t]+#.*$`)

// moved is one step's script moved out: the edit that rewrites the step, and
// the file the step then runs.
type moved struct {
	edit edit.Edit
	// script is the file's path from the checkout root, with forward slashes.
	script string
	text   string
}

// Extract moves every run: script that holds a test out of the workflow at
// path. It answers the workflow and the files the steps now run.
func Extract(path, content string) (string, []fixer.Created) {
	f := fixer.NewFile(path, content, Options(fixer.Options{Creates: true}))
	extractTests(f)
	return f.Text(), f.Report().Created
}

// extractTests moves each run: script that holds a test into a file under
// ScriptDir, and has the step run it. A driver that writes the workflow alone
// gets no change, because the step would run a file nobody wrote.
func extractTests(f *fixer.File) {
	if !f.Creates() {
		return
	}
	root := checkoutRoot(f.Path)
	stem := slug(strings.TrimSuffix(filepath.Base(f.Path), filepath.Ext(f.Path)))
	free := func(rel, text string) bool { return unheld(filepath.Join(root, filepath.FromSlash(rel)), text) }
	tried := set.New[string]()
	for {
		m, ok := nextMove(f.Text(), stem, &tried, free)
		if !ok {
			return
		}
		if res := f.Apply([]edit.Edit{m.edit}); len(res.Applied) == 1 {
			f.Create(filepath.Join(root, filepath.FromSlash(m.script)), m.text)
		}
	}
}

// nextMove answers the move of the earliest step that holds a test and is not
// in tried, and adds that step to tried.
func nextMove(content, stem string, tried *set.Set[string], free func(rel, text string) bool) (moved, bool) {
	for _, s := range shellSteps(content) {
		key := s.job + "/" + strconv.Itoa(s.number)
		if tried.Contains(key) || len(s.block.findings()) == 0 {
			continue
		}
		tried.Add(key)
		if m, ok := s.move(content, stem, free); ok {
			return m, true
		}
	}
	return moved{}, false
}

// checkoutRoot answers the directory a step starts in: the one above
// .github/workflows, or the workflow's own directory.
func checkoutRoot(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) == "workflows" && filepath.Base(filepath.Dir(dir)) == ".github" {
		return filepath.Dir(filepath.Dir(dir))
	}
	return dir
}

// unheld reports whether a script can go at path: nothing is there, or the
// same text is.
func unheld(path, text string) bool {
	held, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	return err == nil && string(held) == text
}

// slug keeps the characters a file name takes as they are, and writes a dash
// for each other one.
func slug(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r == '.' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)
}

// move answers the step with its script moved into a file. It reports false
// when the step's rows or its script cannot be read back exactly.
func (s shellStep) move(content, stem string, free func(rel, text string) bool) (moved, bool) {
	rows := lines(content)
	first, last, suffix, ok := s.valueRows(content, rows)
	if !ok {
		return moved{}, false
	}
	prelude, launch, offset := s.launcher()
	body, args, ok := s.body(offset)
	if !ok {
		return moved{}, false
	}
	text := prelude + body
	base := stem + "-" + slug(s.job) + "-" + strconv.Itoa(s.number)
	rel := ""
	for n := 1; n <= scriptNames && rel == ""; n++ {
		name := base
		if n > 1 {
			name += "-" + strconv.Itoa(n)
		}
		if candidate := ScriptDir + "/" + name + ".sh"; free(candidate, text) {
			rel = candidate
		}
	}
	if rel == "" {
		return moved{}, false
	}
	target := rel
	if s.moved {
		target = `"$GITHUB_WORKSPACE/` + rel + `"`
	}
	command := launch(target)
	for _, arg := range args {
		command += ` "${{ ` + arg + ` }}"`
	}
	value, ok := yamlScalar(command)
	if !ok {
		return moved{}, false
	}
	row := rows[first]
	col := s.run.Column - 1
	return moved{edit: rewrite(content, first, last, []string{row[:col] + value + suffix}), script: rel, text: text}, true
}

// launcher answers the lines the file opens with, the command that runs the
// file, and how many arguments that command passes before the expressions. A
// named shell runs as GitHub runs it. A shell template keeps its own options.
func (s shellStep) launcher() (prelude string, launch func(path string) string, offset int) {
	fields := strings.Fields(s.shell)
	program := "bash"
	if len(fields) > 0 {
		program = fields[0]
	}
	shebang := "#!/usr/bin/env bash\n"
	if program == "sh" {
		shebang = "#!/bin/sh\n"
	}
	if before, after, template := strings.Cut(s.shell, "{0}"); template {
		return shebang, func(path string) string { return before + path + after }, len(strings.Fields(after))
	}
	if program == "sh" {
		return shebang + "set -e\n", func(path string) string { return "sh " + path }, 0
	}
	return shebang + "set -eo pipefail\n", func(path string) string { return "bash " + path }, 0
}

// valueRows answers the rows, counted from zero. That valueRows is that the
// script's YAML value occupies, and the text after the value on its first
// row, such as a comment.
func (s shellStep) valueRows(content string, rows []string) (first, last int, suffix string, ok bool) {
	first = s.run.Line - 1
	col := s.run.Column - 1
	if first < 0 || first >= len(rows) || col < 0 || col > len(rows[first]) || s.step.Style&yaml.FlowStyle != 0 {
		return 0, 0, "", false
	}
	head := rows[first][col:]
	switch s.run.Style {
	case yaml.LiteralStyle, yaml.FoldedStyle:
		if len(s.block.lines) == 0 {
			return 0, 0, "", false
		}
		return first, s.block.start - 2 + len(s.block.lines), trailingComment.FindString(head), true
	}
	last = flowEnd(content, rows, s.run.Line)
	if last < first {
		return 0, 0, "", false
	}
	if last > first {
		return first, last, "", true
	}
	end := scalarEnd(head, s.run.Style)
	if end < 0 {
		return 0, 0, "", false
	}
	return first, last, head[end:], true
}

// flowEnd answers the last row, counted from zero, of a flow scalar that opens
// on a row counted from one. It is the last row before the next node that holds
// more than a comment.
func flowEnd(content string, rows []string, line int) int {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(content), &doc) != nil {
		return -1
	}
	var starts []int
	walk(&doc, func(node *yaml.Node) { starts = append(starts, node.Line) })
	last := nextStart(starts, line, len(rows)) - 1
	for last >= line {
		trimmed := strings.TrimSpace(rows[last])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
		last--
	}
	return last
}

// scalarEnd answers the byte where a scalar written on one row ends in head,
// which opens with the scalar.
func scalarEnd(head string, style yaml.Style) int {
	switch style {
	case yaml.DoubleQuotedStyle:
		for i := 1; i < len(head); i++ {
			switch head[i] {
			case '\\':
				i++
			case '"':
				return i + 1
			}
		}
		return -1
	case yaml.SingleQuotedStyle:
		for i := 1; i < len(head); i++ {
			if head[i] != '\'' {
				continue
			}
			if i+1 < len(head) && head[i+1] == '\'' {
				i++
				continue
			}
			return i + 1
		}
		return -1
	}
	if at := trailingComment.FindStringIndex(head); at != nil {
		return at[0]
	}
	return len(strings.TrimRight(head, " \t"))
}

// yamlScalar writes s as a YAML scalar: plain when it reads back as s, and
// single-quoted when it does not.
func yamlScalar(s string) (string, bool) {
	for _, candidate := range []string{s, "'" + strings.ReplaceAll(s, "'", "''") + "'"} {
		var got map[string]any
		if yaml.Unmarshal([]byte("k: "+candidate+"\n"), &got) == nil && got["k"] == s {
			return candidate, true
		}
	}
	return "", false
}

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
