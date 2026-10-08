package workflow

import (
	"errors"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
	"mvdan.cc/sh/v3/syntax"
)

// IDRunScriptSyntax is a run script that the shell it runs in cannot parse.
const IDRunScriptSyntax = "yaml/run-script-syntax"

// expressionSpan matches a ${{ }} expression, which the runner expands before the shell reads the script.
var expressionSpan = regexp.MustCompile(`(?s)\$\{\{.*?\}\}`)

// shellScript is a run script and what the shell parser says of it.
type shellScript struct {
	run *yaml.Node
	// first is the 1-based file row of the earliest script line, for a literal block.
	first int
	// literal is true when each script line sits on its own file row.
	literal bool
	err     error
}

// runScriptSyntax reports each run script that does not parse as the shell it
// runs in.
func runScriptSyntax(content string) []ste.Finding {
	var out []ste.Finding
	for _, s := range shellScripts(content) {
		if s.err != nil {
			out = append(out, s.finding())
		}
	}
	return out
}

// shellScripts answers every run script whose shell is bash or sh, parsed.
// Unparseable YAML yields nothing, because the runner rejects that file on its own.
func shellScripts(content string) []shellScript {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(content), &doc) != nil {
		return nil
	}
	spans := blockScalars(content)
	var out []shellScript
	for _, s := range runSteps(rootOf(&doc)) {
		run := mappingValue(s.step, "run")
		if run == nil || run.Kind != yaml.ScalarNode {
			continue
		}
		variant, ok := s.variant()
		if !ok {
			continue
		}
		script := shellScript{run: run, first: run.Line}
		if at, held := spans[run.Line]; held && run.Style == yaml.LiteralStyle {
			script.first, script.literal = at.from+1, true
		}
		script.err = parseShell(run.Value, variant)
		out = append(out, script)
	}
	return out
}

// variant answers the shell language a step runs in. ok is false for a shell
// that is neither bash nor sh, such as pwsh, and for a Windows job with no
// shell named. This runs pwsh.
func (s stepRef) variant() (syntax.LangVariant, bool) {
	shell := s.shell
	if own := mappingValue(s.step, "shell"); own != nil {
		shell = own.Value
	} else if s.windows {
		return 0, false
	}
	fields := strings.Fields(shell)
	if len(fields) == 0 {
		return syntax.LangBash, true
	}
	switch fields[0] {
	case "bash":
		return syntax.LangBash, true
	case "sh":
		return syntax.LangPOSIX, true
	}
	return 0, false
}

// parseShell parses a script as the runner hands it to the shell. Each
// expression becomes underscores of its own length, a word that parses as a
// command, an argument, a variable name or a number operand.
func parseShell(script string, variant syntax.LangVariant) error {
	masked := expressionSpan.ReplaceAllStringFunc(script, func(expr string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' {
				return r
			}
			return '_'
		}, expr)
	})
	_, err := syntax.NewParser(syntax.Variant(variant)).Parse(strings.NewReader(masked), "")
	return err
}

func (s shellScript) finding() ste.Finding {
	line, text := s.run.Line, s.err.Error()
	var parse syntax.ParseError
	var lang syntax.LangError
	switch {
	case errors.As(s.err, &parse):
		line, text = s.row(parse.Pos), parse.Text
	case errors.As(s.err, &lang):
		line, text = s.row(lang.Pos), strings.TrimPrefix(lang.Error(), lang.Pos.String()+": ")
	}
	return ste.Finding{
		Line:   line,
		ID:     IDRunScriptSyntax,
		Rule:   "a run script does not parse as shell",
		Detail: text,
		Fix:    "Correct the script so `bash -n` accepts it. A step gated to the default branch runs only after a merge, so no pull request run shows the error.",
	}
}

// row answers the file row a script position sits on. A script that is not a
// literal block is reported on its run: row.
func (s shellScript) row(pos syntax.Pos) int {
	if !s.literal || pos.Line() == 0 {
		return s.run.Line
	}
	return s.first + int(pos.Line()) - 1
}

// failingScripts counts the text of each run script that does not parse.
func failingScripts(content string) map[string]int {
	out := map[string]int{}
	for _, s := range shellScripts(content) {
		if s.err != nil {
			out[s.run.Value]++
		}
	}
	return out
}

// keepScriptsParsing wraps a gate. An edit that leaves a run script failing to
// parse, where its text before the edit parsed, is refused. The script then
// keeps its text. A script that failed before the edit and is left as it was
// does not block an edit elsewhere in the file.
func keepScriptsParsing(gate fixer.Gate) fixer.Gate {
	return func(text string, edits []edit.Edit, scope edit.Scope) edit.Result {
		first := gate(text, edits, scope)
		if len(first.Applied) == 0 {
			return first
		}
		before := failingScripts(text)
		holds := func(out string) bool {
			for script, n := range failingScripts(out) {
				if n > before[script] {
					return false
				}
			}
			return true
		}
		second := edit.Gate(text, first.Applied, scope, func(edit.Edit) string { return "" }, holds)
		for i := range second.Refused {
			second.Refused[i].Reason = "a run script it rewrites no longer parses as shell"
		}
		second.Refused = append(append([]edit.Refused{}, first.Refused...), second.Refused...)
		return second
	}
}
