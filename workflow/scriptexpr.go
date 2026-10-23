// IDScriptExpression is a TypeScript step script that writes a context value in as a ${{ }} expression.

package workflow

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// IDScriptExpression is a TypeScript step script that writes a context value in as a ${{ }} expression.
const IDScriptExpression = "yaml/script-expression"

// typescriptAction matches the org's TypeScript action, which hands the script each context as an object.
var typescriptAction = regexp.MustCompile(`(^|[/@])typescript([@#]|$)`)

// passedContexts are the contexts the TypeScript action reads from a with entry of the same name.
var passedContexts = set.Of("inputs", "steps", "needs", "matrix", "vars")

// githubFields are the fields of the github object that the TypeScript action
// reads from the runner environment with the same value as the expression.
var githubFields = set.Of("sha", "ref", "ref_name", "ref_type", "repository", "repository_owner", "run_id", "run_number",
	"run_attempt", "actor", "triggering_actor", "event_name", "head_ref", "base_ref", "server_url", "api_url", "workflow", "job")

// contextPath matches a dotted read of a context, and captures the root and the rest.
const contextPath = `([a-z]+)((?:\.[A-Za-z_][A-Za-z0-9_-]*)*)`

// scriptExpression matches a ${{ }} expression in a script with the String( and
// the quote around it. A toJSON or a format inside captures the path.
var scriptExpression = regexp.MustCompile(`(String\(\s*)?(["']?)\$\{\{\s*(?:toJSON\(\s*format\(\s*'\{0\}'\s*,\s*` + contextPath +
	`\s*\)\s*\)|toJSON\(\s*` + contextPath + `\s*\)|` + contextPath + `)\s*\}\}(["']?)(\s*\))?`)

// wholePath matches an expression that is one context path.
var wholePath = regexp.MustCompile(`^` + contextPath + `$`)

// identifier matches a segment that a JavaScript dot can read.
var identifier = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// exprRead is one expression on a script line and the read that replaces it.
type exprRead struct {
	start, end int
	expr, text string
	root       string
}

// accessor answers the JavaScript read of a context path, and false for a path
// the script cannot read as the object. A step result output holds JSON the
// github-script action writes, so it stays an expression.
func accessor(root, rest string) (string, bool) {
	segs := strings.Split(strings.TrimPrefix(rest, "."), ".")
	if rest == "" {
		segs = nil
	}
	switch {
	case root == "github":
		if len(segs) != 1 || !githubFields.Contains(segs[0]) {
			return "", false
		}
	case !passedContexts.Contains(root):
		return "", false
	}
	var b strings.Builder
	b.WriteString(root)
	for _, seg := range segs {
		if identifier.MatchString(seg) {
			b.WriteString("." + seg)
			continue
		}
		b.WriteString("[" + strconv.Quote(seg) + "]")
	}
	return b.String(), true
}

// stringRead answers the read for an expression the runner writes as text. A
// string input always holds text. Any other path can be absent, which the
// runner writes as an empty string.
func stringRead(root, rest, read string, strs set.Set[string]) string {
	if root == "inputs" && strs.Contains(strings.TrimPrefix(rest, ".")) {
		return "String(" + read + ")"
	}
	return "String(" + read + ` ?? "")`
}

// objectRead answers the read of the context object that stands in for an env
// expression, and its root. It answers false for an expression that is not one
// path of a context the TypeScript action passes.
func objectRead(expr string, strs set.Set[string]) (string, string, bool) {
	m := wholePath.FindStringSubmatch(expr)
	if m == nil {
		return "", "", false
	}
	read, ok := accessor(m[1], m[2])
	if !ok {
		return "", "", false
	}
	return stringRead(m[1], m[2], read, strs), m[1], true
}

// lineReads answers each expression on a script line that a read of a context
// object can replace.
func lineReads(line string, strs set.Set[string]) []exprRead {
	var out []exprRead
	for _, m := range scriptExpression.FindAllStringSubmatchIndex(line, -1) {
		group := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return line[m[2*i]:m[2*i+1]]
		}
		wrapped := group(1) != "" && group(10) != ""
		start, end := m[0], m[1]
		if !wrapped && group(1) != "" {
			start = m[3]
		}
		if !wrapped && group(10) != "" {
			end = m[20]
		}
		open, closeQuote := group(2), group(9)
		json := group(5) != ""
		quoted := open != "" && open == closeQuote
		var root, rest string
		switch {
		case group(3) != "":
			root, rest = group(3), group(4)
		case json:
			root, rest = group(5), group(6)
		default:
			root, rest = group(7), group(8)
		}
		// toJSON writes its own quotes. A bare path is text only inside a quote.
		if json && (open != "" || closeQuote != "") || !json && open+closeQuote != "" && !quoted || group(7) != "" && !quoted {
			continue
		}
		read, ok := accessor(root, rest)
		if !ok {
			continue
		}
		text := stringRead(root, rest, read, strs)
		if json && !wrapped && !(root == "inputs" && strs.Contains(strings.TrimPrefix(rest, "."))) {
			text = read
		}
		out = append(out, exprRead{start: start, end: end, expr: line[start:end], text: text, root: root})
	}
	return out
}

// passEntry is the with value that hands a context to the script.
func passEntry(root string) string {
	return "${{ toJSON(" + root + ") }}"
}

// passes reports whether the script can read root as an object. The action
// builds github itself, and with passes any other context or can take a line
// that does. A with entry that names the context another way blocks it.
func passes(with *yaml.Node, root string) bool {
	if root == "github" {
		return true
	}
	if have := mappingValue(with, root); have != nil {
		return strings.Join(strings.Fields(have.Value), "") == strings.Join(strings.Fields(passEntry(root)), "")
	}
	return with != nil && with.Style&yaml.FlowStyle == 0 && mappingKey(with, "script") != nil
}

// withEntries answers the lines that hand each root with lacks to the script,
// and the row of the script key they go above.
func withEntries(with *yaml.Node, roots set.Set[string], fileLines []string) ([]string, int) {
	scriptKey := mappingKey(with, "script")
	row := scriptKey.Line - 1
	indent := fileLines[row][:scriptKey.Column-1]
	var names []string
	for root := range roots.All() {
		if root != "github" && mappingValue(with, root) == nil {
			names = append(names, root)
		}
	}
	sort.Strings(names)
	var out []string
	for _, root := range names {
		out = append(out, indent+root+": "+passEntry(root))
	}
	return out, row
}

// addEntries writes entries above the file row, ahead of any rewrite of that row.
func addEntries(rows map[int][]string, row int, entries []string, fileLines []string) {
	if len(entries) == 0 {
		return
	}
	keep, held := rows[row]
	if !held {
		keep = []string{fileLines[row]}
	}
	rows[row] = append(entries, keep...)
}

// rowEdits answers an edit for each rewritten row, in file order.
func rowEdits(content string, rows map[int][]string) []edit.Edit {
	var out []edit.Edit
	for row, text := range rows {
		out = append(out, rewrite(content, row, row, text))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// scriptExpressions reports each expression a TypeScript script reads where the
// context object holds the value.
func scriptExpressions(content string) []ste.Finding {
	var out []ste.Finding
	for _, p := range scriptExprPlans(content) {
		out = append(out, p.findings...)
	}
	return out
}

// readContexts writes each read of a context object over its expression.
func readContexts(content string) []edit.Edit {
	var out []edit.Edit
	for _, p := range scriptExprPlans(content) {
		out = append(out, p.edits...)
	}
	return out
}

func scriptExprPlans(content string) []envPlan {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(content), &doc) != nil {
		return nil
	}
	fileLines := lines(content)
	spans := blockScalars(content)
	root := rootOf(&doc)
	strs := stringInputs(root)
	var out []envPlan
	for _, s := range runSteps(root) {
		if p := scriptExprPlan(s.step, content, fileLines, spans, strs); len(p.findings) > 0 {
			out = append(out, p)
		}
	}
	return out
}

func scriptExprPlan(step *yaml.Node, content string, fileLines []string, spans map[int]lineSpan, strs set.Set[string]) envPlan {
	var p envPlan
	uses := mappingValue(step, "uses")
	with := mappingValue(step, "with")
	script := mappingValue(with, "script")
	if uses == nil || !typescriptAction.MatchString(uses.Value) || script == nil || script.Kind != yaml.ScalarNode {
		return p
	}
	at, ok := scalarLines(script, fileLines, spans)
	if !ok {
		return p
	}
	roots := set.New[string]()
	rows := map[int][]string{}
	var findings []ste.Finding
	for k := range at.file {
		body := at.file[k][len(at.prefix[k]):]
		var reads []exprRead
		for _, r := range lineReads(body, strs) {
			if passes(with, r.root) {
				reads = append(reads, r)
			}
		}
		if len(reads) == 0 {
			continue
		}
		var b strings.Builder
		b.WriteString(at.prefix[k])
		last := 0
		for _, r := range reads {
			roots.Add(r.root)
			b.WriteString(body[last:r.start])
			b.WriteString(r.text)
			last = r.end
			findings = append(findings, ste.Finding{
				Line: at.lineNos[k] + 1, ID: IDScriptExpression,
				Rule:   "the script writes a context value in as an expression, where it can read the context object",
				Detail: r.expr,
				Fix:    "Write " + r.text + " in its place, and pass the context under with.",
			})
		}
		b.WriteString(body[last:])
		rows[at.lineNos[k]] = []string{b.String()}
	}
	if len(findings) == 0 {
		return p
	}
	entries, row := withEntries(with, roots, fileLines)
	addEntries(rows, row, entries, fileLines)
	p.findings = findings
	p.edits = rowEdits(content, rows)
	return p
}
