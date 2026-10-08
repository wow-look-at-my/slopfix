package workflow

import (
	"regexp"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
	"mvdan.cc/sh/v3/syntax"
)

// IDEnvIndirection is the rule that finds a step env entry with no job to do.
const IDEnvIndirection = "yaml/env-indirection"

// wholeExpression matches a value that is one ${{ }} expression and nothing else.
var wholeExpression = regexp.MustCompile(`^\$\{\{\s*([^}]*?)\s*\}\}$`)

// typedInputs are the input types whose value cannot carry shell text.
var typedInputs = set.Of("boolean", "number")

// safeInputs names the inputs every trigger of a workflow types as a boolean or
// a number. A string input, a choice, an untyped input and every input of a
// composite action carry text the caller writes.
func safeInputs(root *yaml.Node) set.Set[string] {
	safe := set.New[string]()
	unsafe := set.New[string]()
	if mappingValue(root, "jobs") == nil {
		return safe
	}
	on := mappingValue(root, "on")
	for _, trigger := range []string{"workflow_dispatch", "workflow_call"} {
		inputs := mappingValue(mappingValue(on, trigger), "inputs")
		if inputs == nil || inputs.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(inputs.Content); i += 2 {
			name := inputs.Content[i].Value
			kind := mappingValue(inputs.Content[i+1], "type")
			if kind != nil && typedInputs.Contains(kind.Value) {
				safe.Add(name)
				continue
			}
			unsafe.Add(name)
		}
	}
	return safe.Difference(unsafe)
}

// plainContext matches a context whose value holds no quote, space or other
// shell text: a runner fact, a number, a hash or a name GitHub restricts.
var plainContext = regexp.MustCompile(`^(?:runner\.(?:temp|tool_cache|os|arch|name|environment|debug)|` +
	`github\.(?:event_name|sha|workflow_sha|run_id|run_number|run_attempt|ref_type|repository|repository_owner|repository_id|repository_owner_id|actor_id|server_url|api_url|graphql_url|retention_days|job)|` +
	`strategy\.(?:job-index|job-total|fail-fast|max-parallel)|job\.status)$`)

// typedInput matches a read of a single input by its name.
var typedInput = regexp.MustCompile(`^inputs\.([A-Za-z_][A-Za-z0-9_-]*)$`)

// plainValue reports an expression whose value cannot carry shell text.
func plainValue(expr string, safe set.Set[string]) bool {
	expr = strings.TrimSpace(expr)
	if m := typedInput.FindStringSubmatch(expr); m != nil {
		return safe.Contains(m[1])
	}
	return plainContext.MatchString(expr)
}

// runnerContext maps a runner variable to the context that carries the same
// value. A path in this map names the host inside a job container, where the
// variable names the mount. A step that can run in a container keeps the variable.
var runnerContext = map[string]string{
	"RUNNER_TEMP":        "runner.temp",
	"RUNNER_TOOL_CACHE":  "runner.tool_cache",
	"RUNNER_OS":          "runner.os",
	"RUNNER_ARCH":        "runner.arch",
	"RUNNER_NAME":        "runner.name",
	"RUNNER_ENVIRONMENT": "runner.environment",
}

// envStep is a run step that this rule can read: its script parses as shell,
// and its rows sit where the file holds them.
type envStep struct {
	step      *yaml.Node
	container bool
	script    string
	// rows maps each script line to its 0-based file row.
	rows []int
	// file holds each file row of the script, for the rewrite.
	file []string
	// prefix is the bytes on each row before the script line starts.
	prefix []string
	parsed *syntax.File
	// safe names the inputs whose value cannot carry shell text.
	safe set.Set[string]
}

// envPlan is what the rule finds in one step, and the edits that repair it.
type envPlan struct {
	findings []ste.Finding
	edits    []edit.Edit
}

// envIndirections reports each step env entry that a run script could do
// without, and each runner variable that a context names.
func envIndirections(content string) []ste.Finding {
	var out []ste.Finding
	for _, p := range envPlans(content) {
		out = append(out, p.findings...)
	}
	return out
}

// inlineEnv applies every plan the rule finds.
func inlineEnv(content string) []edit.Edit {
	var out []edit.Edit
	for _, p := range envPlans(content) {
		out = append(out, p.edits...)
	}
	return out
}

func envPlans(content string) []envPlan {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(content), &doc) != nil {
		return nil
	}
	rows := lines(content)
	spans := blockScalars(content)
	var out []envPlan
	root := rootOf(&doc)
	safe := safeInputs(root)
	for _, s := range runSteps(root) {
		es, ok := readEnvStep(s.step, s.container, s.windows, rows, spans)
		if !ok {
			continue
		}
		es.safe = safe
		if p := es.plan(content, rows); len(p.findings) > 0 {
			out = append(out, p)
		}
	}
	return out
}

// stepRef is a step with what its job says about the shell it runs in.
type stepRef struct {
	step      *yaml.Node
	container bool
	windows   bool
}

// runSteps answers every step of every job, and every step of a composite action.
func runSteps(root *yaml.Node) []stepRef {
	var out []stepRef
	if jobs := mappingValue(root, "jobs"); jobs != nil {
		rootShell := defaultShell(root)
		for i := 0; i+1 < len(jobs.Content); i += 2 {
			job := jobs.Content[i+1]
			steps := mappingValue(job, "steps")
			if steps == nil || steps.Kind != yaml.SequenceNode {
				continue
			}
			shell := defaultShell(job)
			if shell == "" {
				shell = rootShell
			}
			windows := false
			if on := mappingValue(job, "runs-on"); on != nil && shell == "" {
				var text strings.Builder
				walk(on, func(n *yaml.Node) { text.WriteString(n.Value) })
				windows = strings.Contains(strings.ToLower(text.String()), "windows")
			}
			for _, step := range steps.Content {
				out = append(out, stepRef{step: step, container: mappingValue(job, "container") != nil, windows: windows || !bashLike(shell)})
			}
		}
	}
	if steps := mappingValue(mappingValue(root, "runs"), "steps"); steps != nil && steps.Kind == yaml.SequenceNode {
		for _, step := range steps.Content {
			// A composite action does not know the job that calls it. That job can run in a container.
			out = append(out, stepRef{step: step, container: true})
		}
	}
	return out
}

func defaultShell(node *yaml.Node) string {
	if shell := mappingValue(mappingValue(mappingValue(node, "defaults"), "run"), "shell"); shell != nil {
		return shell.Value
	}
	return ""
}

// bashLike reports whether a shell value runs a POSIX shell.
func bashLike(shell string) bool {
	if shell == "" {
		return true
	}
	first := strings.Fields(shell)[0]
	return first == "bash" || first == "sh"
}

func readEnvStep(step *yaml.Node, container, windows bool, rows []string, spans map[int]rowSpan) (envStep, bool) {
	run := mappingValue(step, "run")
	if run == nil || run.Kind != yaml.ScalarNode {
		return envStep{}, false
	}
	if shell := mappingValue(step, "shell"); shell != nil {
		if !bashLike(shell.Value) {
			return envStep{}, false
		}
	} else if windows {
		return envStep{}, false
	}
	es := envStep{step: step, container: container, script: run.Value}
	switch run.Style {
	case yaml.LiteralStyle:
		at, held := spans[run.Line]
		if !held {
			return envStep{}, false
		}
		script := strings.Split(strings.TrimSuffix(run.Value, "\n"), "\n")
		if at.from+len(script) > len(rows) {
			return envStep{}, false
		}
		for k, line := range script {
			row := rows[at.from+k]
			if !strings.HasSuffix(row, line) {
				return envStep{}, false
			}
			es.rows = append(es.rows, at.from+k)
			es.file = append(es.file, row)
			es.prefix = append(es.prefix, row[:len(row)-len(line)])
		}
	case 0:
		if strings.Contains(run.Value, "\n") || run.Line-1 >= len(rows) {
			return envStep{}, false
		}
		row := rows[run.Line-1]
		col := run.Column - 1
		if col < 0 || col > len(row) || strings.TrimRight(row[col:], " \t") != run.Value {
			return envStep{}, false
		}
		es.rows = []int{run.Line - 1}
		es.file = []string{row}
		es.prefix = []string{row[:col]}
	default:
		return envStep{}, false
	}
	parsed, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(es.script), "")
	if err != nil {
		return envStep{}, false
	}
	es.parsed = parsed
	return es, true
}

// paramRef is one expansion of a variable in the script.
type paramRef struct {
	start, end int
	// plain is true for $NAME and ${NAME}, which a value can stand in for.
	plain bool
}

func (es envStep) refs() map[string][]paramRef {
	out := map[string][]paramRef{}
	syntax.Walk(es.parsed, func(node syntax.Node) bool {
		p, ok := node.(*syntax.ParamExp)
		if !ok || p.Param == nil {
			return true
		}
		plain := !p.Excl && !p.Length && !p.Width && p.Index == nil && p.Slice == nil && p.Repl == nil && p.Names == 0 && p.Exp == nil
		out[p.Param.Value] = append(out[p.Param.Value], paramRef{start: int(p.Pos().Offset()), end: int(p.End().Offset()), plain: plain})
		return true
	})
	return out
}

// overwrites reports whether the script assigns name before anything can read
// it. Every statement before the assignment is a set builtin, or an
// assignment that does not read name.
func (es envStep) overwrites(name string, refs []paramRef) bool {
	for _, stmt := range es.parsed.Stmts {
		call, ok := stmt.Cmd.(*syntax.CallExpr)
		if !ok || stmt.Negated || stmt.Background || len(stmt.Redirs) > 0 {
			return false
		}
		if readsWithin(refs, stmt) {
			return false
		}
		if len(call.Args) == 0 {
			for _, as := range call.Assigns {
				if as.Name != nil && as.Name.Value == name && !as.Append && as.Index == nil {
					return true
				}
			}
			continue
		}
		if len(call.Args) > 0 && call.Args[0].Lit() == "set" {
			continue
		}
		return false
	}
	return false
}

func readsWithin(refs []paramRef, node syntax.Node) bool {
	from, to := int(node.Pos().Offset()), int(node.End().Offset())
	for _, r := range refs {
		if r.start >= from && r.end <= to {
			return true
		}
	}
	return false
}

// replacement is a span of the script and the text that stands in for it.
type replacement struct {
	start, end int
	text       string
}

func (es envStep) plan(content string, rows []string) envPlan {
	var p envPlan
	refs := es.refs()
	var swaps []replacement
	drop := set.New[int]()
	envKey := mappingKey(es.step, "env")
	env := mappingValue(es.step, "env")
	kept := 0
	if env != nil && env.Kind == yaml.MappingNode && env.Style&yaml.FlowStyle == 0 {
		others := stepTextOutsideRun(es.step)
		for i := 0; i+1 < len(env.Content); i += 2 {
			key, value := env.Content[i], env.Content[i+1]
			name := key.Value
			found, inline := es.judgeEntry(name, key, value, refs[name], others, rows)
			if !found {
				kept++
				continue
			}
			p.findings = append(p.findings, found2finding(key.Line, name, value.Value, inline))
			drop.Add(key.Line - 1)
			if inline {
				expr := strings.TrimSpace(value.Value)
				for _, r := range refs[name] {
					swaps = append(swaps, replacement{r.start, r.end, expr})
				}
			}
		}
		if kept == 0 && len(p.findings) > 0 && envKey != nil && envKey.Line != env.Content[0].Line {
			drop.Add(envKey.Line - 1)
		}
	}
	if !es.container {
		swaps = append(swaps, es.runnerSwaps(&p, refs, env)...)
	}
	p.edits = append(p.edits, es.rewriteScript(content, swaps)...)
	p.edits = append(p.edits, dropRows(content, drop)...)
	return p
}

// judgeEntry reports whether an env entry has no job to do, and whether its
// expression goes into the script in its place. An entry the script overwrites
// before it reads it carries a value nothing reads.
func (es envStep) judgeEntry(name string, key, value *yaml.Node, refs []paramRef, others string, rows []string) (found, inline bool) {
	if value.Kind != yaml.ScalarNode || value.Line != key.Line || value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return false, false
	}
	if strings.Contains(others, "env."+name) || len(refs) == 0 {
		return false, false
	}
	if es.overwrites(name, refs) {
		return true, false
	}
	match := wholeExpression.FindStringSubmatch(strings.TrimSpace(value.Value))
	if match == nil || !plainValue(match[1], es.safe) {
		return false, false
	}
	for _, r := range refs {
		if !r.plain {
			return false, false
		}
	}
	for _, as := range assignsOf(es.parsed) {
		if as == name {
			return false, false
		}
	}
	return true, true
}

func found2finding(line int, name, value string, inline bool) ste.Finding {
	if inline {
		return ste.Finding{
			Line: line, ID: IDEnvIndirection,
			Rule:   "an env entry only carries an expression into the script",
			Detail: name + ": " + value,
			Fix:    "Write the expression in the script where it reads $" + name + ", and remove the entry.",
		}
	}
	return ste.Finding{
		Line: line, ID: IDEnvIndirection,
		Rule:   "the script sets this variable before it reads it",
		Detail: name + ": " + value,
		Fix:    "Nothing reads this value. Remove the entry.",
	}
}

func (es envStep) runnerSwaps(p *envPlan, refs map[string][]paramRef, env *yaml.Node) []replacement {
	var out []replacement
	assigned := set.Of(assignsOf(es.parsed)...)
	names := make([]string, 0, len(runnerContext))
	for name := range runnerContext {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if assigned.Contains(name) || mappingValue(env, name) != nil {
			continue
		}
		for _, r := range refs[name] {
			if !r.plain {
				continue
			}
			expr := "${{ " + runnerContext[name] + " }}"
			out = append(out, replacement{r.start, r.end, expr})
			p.findings = append(p.findings, ste.Finding{
				Line: es.rows[0] + 1 + strings.Count(es.script[:r.start], "\n"), ID: IDEnvIndirection,
				Rule:   "the script reads a runner variable that a context names",
				Detail: "$" + name,
				Fix:    "Write " + expr + " in its place.",
			})
		}
	}
	return out
}

func assignsOf(f *syntax.File) []string {
	var out []string
	syntax.Walk(f, func(node syntax.Node) bool {
		if as, ok := node.(*syntax.Assign); ok && as.Name != nil {
			out = append(out, as.Name.Value)
		}
		return true
	})
	return out
}

// stepTextOutsideRun answers every scalar of the step except its script, so an
// env.NAME in an if or a with keeps the entry.
func stepTextOutsideRun(step *yaml.Node) string {
	run := mappingValue(step, "run")
	var out strings.Builder
	walk(step, func(n *yaml.Node) {
		if n != run && n.Kind == yaml.ScalarNode {
			out.WriteString(n.Value)
			out.WriteByte('\n')
		}
	})
	return out.String()
}

// rewriteScript splices the replacements into the script, and rewrites each row
// whose line changed.
func (es envStep) rewriteScript(content string, swaps []replacement) []edit.Edit {
	if len(swaps) == 0 {
		return nil
	}
	sort.Slice(swaps, func(i, j int) bool { return swaps[i].start < swaps[j].start })
	var b strings.Builder
	at := 0
	for _, s := range swaps {
		if s.start < at {
			continue
		}
		b.WriteString(es.script[at:s.start])
		b.WriteString(s.text)
		at = s.end
	}
	b.WriteString(es.script[at:])
	before := strings.Split(strings.TrimSuffix(es.script, "\n"), "\n")
	after := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")
	if len(before) != len(after) {
		return nil
	}
	var out []edit.Edit
	for k := range before {
		if before[k] == after[k] || k >= len(es.rows) {
			continue
		}
		line := es.prefix[k] + after[k]
		if k == 0 && len(es.rows) == 1 {
			line += strings.TrimPrefix(es.file[0], es.prefix[0]+before[0])
		}
		out = append(out, rewrite(content, es.rows[k], es.rows[k], []string{line}))
	}
	return out
}
