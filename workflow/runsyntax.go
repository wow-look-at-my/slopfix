package workflow

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
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
	// variant is the shell language the script was parsed as.
	variant syntax.LangVariant
	// shell is the node that names the shell, the step's own or the default it inherits.
	shell *yaml.Node
	err   error
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
		script := shellScript{run: run, first: run.Line, variant: variant, shell: s.shellNode}
		if own := mappingValue(s.step, "shell"); own != nil {
			script.shell = own
		}
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
		Fix:    "Correct the script so `bash -n` accepts it. `slopfix fix` gives an empty body the no-op `:`, closes what the script left open, drops a stray closing word, quotes a stray operator, passes any other rejected line to `eval`, and names bash for an `sh` step that uses a bash feature.",
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

// closerError matches a parse error that names the word that closes what the script opened: a statement that must end with it.
var closerError = regexp.MustCompile("(?:statement must end with|without matching .* with|without closing quote) `([^`]+)`$")

// emptyBodyError matches a keyword that opens a body and finds none.
var emptyBodyError = regexp.MustCompile("^`([^`]+)` must be followed by a statement list$")

// repairScript answers the script with each syntax error mended. A keyword
// with an empty body gets the no-op `:`. An unclosed statement, bracket or
// quote gets its closer at the end.
func repairScript(script string, variant syntax.LangVariant) string {
	for range strings.Count(script, "\n") + 2 {
		err := parseShell(script, variant)
		if err == nil {
			return script
		}
		script = mendScript(script, err)
	}
	return script
}

// strayWordError matches a closing or continuing word that has nothing to close or continue.
var strayWordError = regexp.MustCompile("^`([^`!]+)` can only be used ")

// operatorRunes are the characters the shell reads as operators, not as parts of a word.
const operatorRunes = "()<>|&;"

// mendScript changes script at the place err names. Every command the script
// holds stays in it: an empty body gets `:`. An unclosed construct gets its
// closer, a stray closing word is dropped. A stray operator is quoted into a
// word, and any other rejected line runs through `eval`. This fails loudly
// at run time with the line it was given.
func mendScript(script string, err error) string {
	var parse syntax.ParseError
	if !errors.As(err, &parse) {
		return script
	}
	if body := emptyBodyError.FindStringSubmatch(parse.Text); body != nil {
		if mended, ok := insertAfter(script, parse.Pos, body[1], " :"); ok {
			return mended
		}
	}
	if closer := closerError.FindStringSubmatch(parse.Text); closer != nil {
		return strings.TrimSuffix(script, "\n") + "\n" + closer[1] + "\n"
	}
	if stray := strayWordError.FindStringSubmatch(parse.Text); stray != nil {
		if mended, ok := removeAt(script, parse.Pos, stray[1]); ok {
			return mended
		}
	}
	if mended, ok := quoteOperator(script, parse.Pos); ok {
		return mended
	}
	// A word followed by ( reads as a function name. The error names the word, so the bracket is the next one on its line.
	if parse.Text == "`foo(` must be followed by `)`" {
		line, col := int(parse.Pos.Line())-1, int(parse.Pos.Col())-1
		rows := strings.Split(script, "\n")
		if line >= 0 && line < len(rows) && col >= 0 && col <= len(rows[line]) {
			if at := strings.Index(rows[line][col:], "("); at >= 0 {
				if mended, ok := quoteOperatorAt(script, line, col+at); ok {
					return mended
				}
			}
		}
	}
	return evalLine(script, int(parse.Pos.Line()))
}

// removeAt deletes word where it starts at pos.
func removeAt(script string, pos syntax.Pos, word string) (string, bool) {
	rows := strings.Split(script, "\n")
	line, col := int(pos.Line())-1, int(pos.Col())-1
	if line < 0 || line >= len(rows) || col < 0 || col > len(rows[line]) || !strings.HasPrefix(rows[line][col:], word) {
		return script, false
	}
	rows[line] = rows[line][:col] + rows[line][col+len(word):]
	return strings.Join(rows, "\n"), true
}

// quoteOperator wraps the run of operator characters at pos in single quotes,
// which makes it an argument.
func quoteOperator(script string, pos syntax.Pos) (string, bool) {
	return quoteOperatorAt(script, int(pos.Line())-1, int(pos.Col())-1)
}

// quoteOperatorAt is quoteOperator for a 0-based line and byte column.
func quoteOperatorAt(script string, line, col int) (string, bool) {
	rows := strings.Split(script, "\n")
	if line < 0 || line >= len(rows) || col < 0 || col >= len(rows[line]) || !strings.ContainsRune(operatorRunes, rune(rows[line][col])) {
		return script, false
	}
	end := col
	for end < len(rows[line]) && strings.ContainsRune(operatorRunes, rune(rows[line][end])) {
		end++
	}
	rows[line] = rows[line][:col] + "'" + rows[line][col:end] + "'" + rows[line][end:]
	return strings.Join(rows, "\n"), true
}

// evalLine passes the 1-based line of script to eval as one quoted word.
func evalLine(script string, line int) string {
	rows := strings.Split(script, "\n")
	line = min(max(line, 1), len(rows)) - 1
	for line > 0 && strings.TrimSpace(rows[line]) == "" {
		line--
	}
	body := strings.TrimLeft(rows[line], " \t")
	rows[line] = rows[line][:len(rows[line])-len(body)] + "eval '" + strings.ReplaceAll(body, "'", `'\''`) + "'"
	return strings.Join(rows, "\n")
}

// insertAfter writes text right after the keyword that starts at pos.
func insertAfter(script string, pos syntax.Pos, keyword, text string) (string, bool) {
	rows := strings.Split(script, "\n")
	line, col := int(pos.Line())-1, int(pos.Col())-1
	if line < 0 || line >= len(rows) || col < 0 || col > len(rows[line]) || !strings.HasPrefix(rows[line][col:], keyword) {
		return script, false
	}
	at := col + len(keyword)
	rows[line] = rows[line][:at] + text + rows[line][at:]
	return strings.Join(rows, "\n"), true
}

// nameBash writes bash in place of the leading sh of the shell value on its row.
func nameBash(rows []string, shell *yaml.Node) (string, bool) {
	row, col := shell.Line-1, shell.Column-1
	if row < 0 || row >= len(rows) || col < 0 || col > len(rows[row]) {
		return "", false
	}
	at := strings.Index(rows[row][col:], "sh")
	if at < 0 {
		return "", false
	}
	at += col
	return rows[row][:at] + "bash" + rows[row][at+len("sh"):], true
}

// repairRunScripts writes each run script that does not parse as shell back
// as the repairScript text. A literal or folded block takes the rows its text
// held. A scalar on one row is written again as a quoted string.
func repairRunScripts(content string) []edit.Edit {
	rows := lines(content)
	spans := blockScalars(content)
	var out []edit.Edit
	named := map[int]bool{}
	for _, s := range shellScripts(content) {
		if s.err == nil {
			continue
		}
		variant := s.variant
		var lang syntax.LangError
		if errors.As(s.err, &lang) && s.shell != nil {
			variant = syntax.LangBash
			if row := s.shell.Line - 1; !named[row] {
				named[row] = true
				if edited, ok := nameBash(rows, s.shell); ok {
					out = append(out, rewrite(content, row, row, []string{edited}))
				}
			}
		}
		mended := repairScript(s.run.Value, variant)
		if mended == s.run.Value {
			continue
		}
		if at, held := spans[s.run.Line]; held {
			indent := ""
			if at.from < len(rows) {
				body := strings.TrimLeft(rows[at.from], " ")
				indent = rows[at.from][:len(rows[at.from])-len(body)]
			}
			var written []string
			for _, row := range strings.Split(strings.TrimSuffix(mended, "\n"), "\n") {
				if row == "" {
					written = append(written, "")
					continue
				}
				written = append(written, indent+row)
			}
			out = append(out, rewrite(content, at.from, at.to-1, written))
			continue
		}
		row := s.run.Line - 1
		col := s.run.Column - 1
		if row < 0 || row >= len(rows) || col < 0 || col > len(rows[row]) || strings.Contains(rows[row][col:], " #") {
			continue
		}
		out = append(out, rewrite(content, row, row, []string{rows[row][:col] + strconv.Quote(mended)}))
	}
	return out
}
