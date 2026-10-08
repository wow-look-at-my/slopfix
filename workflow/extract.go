// extract.go moves a run: script that holds a test into a file of its own.
//
// A line of a run: script is shell. A cut of the line that asserts can leave
// an if with an empty then, so the repair cuts no line. The whole script moves
// into a file, and the step runs the file.
package workflow

import (
	"errors"
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
)

// ScriptDir is where a moved script lives, from the checkout root.
const ScriptDir = ".github/scripts"

// scriptNames bounds the suffixes tried for a script name another file holds.
const scriptNames = 100

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
