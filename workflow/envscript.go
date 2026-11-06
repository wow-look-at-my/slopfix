// placedLines are the file lines a scalar's text sits on.

package workflow

import (
	"regexp"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// placedLines are the file lines a scalar's text sits on.
type placedLines struct {
	// lineNos maps each line of the scalar to its 0-based file line.
	lineNos []int
	// file holds each file line of the scalar.
	file []string
	// prefix is the bytes on each line before the scalar's line starts.
	prefix []string
}

// scalarLines answers where each line of a literal or a single-line plain
// scalar sits in the file. Any other style, or text the file does not hold
// verbatim, answers false.
func scalarLines(node *yaml.Node, fileLines []string, spans map[int]lineSpan) (placedLines, bool) {
	var out placedLines
	switch node.Style {
	case yaml.LiteralStyle:
		at, held := spans[node.Line]
		if !held {
			return placedLines{}, false
		}
		text := strings.Split(strings.TrimSuffix(node.Value, "\n"), "\n")
		if at.from+len(text) > len(fileLines) {
			return placedLines{}, false
		}
		for k, line := range text {
			fileLine := fileLines[at.from+k]
			if !strings.HasSuffix(fileLine, line) {
				return placedLines{}, false
			}
			out.lineNos = append(out.lineNos, at.from+k)
			out.file = append(out.file, fileLine)
			out.prefix = append(out.prefix, fileLine[:len(fileLine)-len(line)])
		}
	case 0:
		if strings.Contains(node.Value, "\n") || node.Line-1 >= len(fileLines) {
			return placedLines{}, false
		}
		fileLine := fileLines[node.Line-1]
		col := node.Column - 1
		if col < 0 || col > len(fileLine) || strings.TrimRight(fileLine[col:], " \t") != node.Value {
			return placedLines{}, false
		}
		out.lineNos = []int{node.Line - 1}
		out.file = []string{fileLine}
		out.prefix = []string{fileLine[:col]}
	default:
		return placedLines{}, false
	}
	return out, true
}

// processEnv matches every mention of process.env.
var processEnv = regexp.MustCompile(`\bprocess\s*\.\s*env\b`)

// envMember matches a member read of process.env, and captures the name.
var envMember = regexp.MustCompile(`\bprocess\.env\.([A-Za-z_$][A-Za-z0-9_$]*)`)

// envMemberAt matches a member read of process.env at the start of the text.
var envMemberAt = regexp.MustCompile(`^process\.env\.[A-Za-z_$]`)

// writesAfter matches what follows a read that writes the variable.
var writesAfter = regexp.MustCompile(`^\s*(\?\?=|\+\+|--|=($|[^=])|[-+*/%&|^]=|&&=|\|\|=|<<=|>>=)`)

// writesBefore matches what precedes a read that deletes or steps the variable.
var writesBefore = regexp.MustCompile(`(\bdelete|\+\+|--)\s*$`)

// identChar reports whether b continues a JavaScript identifier.
func identChar(b byte) bool {
	return b == '_' || b == '$' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// mentions answers each place a script names name as a whole identifier.
func mentions(script, name string) int {
	count := 0
	if name == "" {
		return count
	}
	for at := 0; ; {
		i := strings.Index(script[at:], name)
		if i < 0 {
			return count
		}
		start, end := at+i, at+i+len(name)
		if (start == 0 || !identChar(script[start-1])) && (end == len(script) || !identChar(script[end])) {
			count++
		}
		at = end
	}
}

// shellReads answers each place text reads name as a shell expansion, $NAME or
// ${NAME}. A shell step reads the environment that way for both forms.
func shellReads(text, name string) int {
	count := 0
	for at := 0; at < len(text); {
		i := strings.IndexByte(text[at:], '$')
		if i < 0 {
			return count
		}
		start := at + i
		rest := strings.TrimPrefix(text[start+1:], "{")
		if strings.HasPrefix(rest, name) {
			if after := rest[len(name):]; after == "" || !identChar(after[0]) {
				count++
			}
		}
		at = start + 1
	}
	return count
}

// exactReads answers each process.env.NAME read on a line. A read takes in
// the dead `??` arms after it, because the entry always defines the variable.
// The second result is false when a read writes the variable or deletes it.
func exactReads(line, name string) ([]envRead, bool) {
	var out []envRead
	for _, m := range envMember.FindAllStringSubmatchIndex(line, -1) {
		if line[m[2]:m[3]] != name {
			continue
		}
		if writesAfter.MatchString(line[m[1]:]) || writesBefore.MatchString(line[:m[0]]) {
			return nil, false
		}
		out = append(out, envRead{start: m[0], end: deadArms(line, m[1])})
	}
	return out, true
}

// wholeEnv reports whether a script hands process.env on whole, to a child
// process or a spread, where no member read shows what it reads.
func wholeEnv(script string) bool {
	for _, m := range processEnv.FindAllStringIndex(script, -1) {
		if !envMemberAt.MatchString(script[m[0]:]) {
			return true
		}
	}
	return false
}

// stringInputs names the inputs whose value is always a string. Every input of
// a composite action, and each workflow input typed as text on every trigger.
func stringInputs(root *yaml.Node) set.Set[string] {
	out := set.New[string]()
	if mappingValue(root, "runs") != nil {
		inputs := mappingValue(root, "inputs")
		if inputs != nil && inputs.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(inputs.Content); i += 2 {
				out.Add(inputs.Content[i].Value)
			}
		}
		return out
	}
	other := set.New[string]()
	on := mappingValue(root, "on")
	for _, trigger := range []string{"workflow_dispatch", "workflow_call"} {
		inputs := mappingValue(mappingValue(on, trigger), "inputs")
		if inputs == nil || inputs.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(inputs.Content); i += 2 {
			kind := mappingValue(inputs.Content[i+1], "type")
			if kind == nil || kind.Value == "string" || kind.Value == "choice" || kind.Value == "environment" {
				out.Add(inputs.Content[i].Value)
				continue
			}
			other.Add(inputs.Content[i].Value)
		}
	}
	return out.Difference(other)
}

// inputName matches an expression that is one input read by name.
var inputName = regexp.MustCompile(`^inputs\.([A-Za-z_][A-Za-z0-9_-]*)$`)

// readOf answers what the script reads in place of an env value, and the
// context the step must pass for it. A TypeScript step reads the context object
// that holds the value. Any other expression goes in through toJSON, which
// quotes any text safely. An expression that can yield a number, a boolean or
// null is formatted first, as the runner formats an env value.
func readOf(expr string, strs set.Set[string], with *yaml.Node) (string, string) {
	if with != nil {
		if read, root, ok := objectRead(expr, strs); ok && passes(with, root) {
			return read, root
		}
	}
	if m := inputName.FindStringSubmatch(expr); m != nil && strs.Contains(m[1]) {
		return "String(${{ toJSON(" + expr + ") }})", ""
	}
	return "String(${{ toJSON(format('{0}', " + expr + ")) }})", ""
}

// stepsText answers every scalar of every step but one, so a read of a name in
// another step keeps the entry.
func stepsText(all []stepRef, except *yaml.Node) string {
	var out strings.Builder
	for _, s := range all {
		if s.step == except {
			continue
		}
		walk(s.step, func(n *yaml.Node) {
			if n.Kind == yaml.ScalarNode {
				out.WriteString(n.Value)
				out.WriteByte('\n')
			}
		})
	}
	return out.String()
}

// scriptPlan judges a step of a script action, whose with.script reads each
// env entry as process.env.NAME. An entry that only carries an expression in
// goes, and the expression takes the place of each read.
func scriptPlan(s stepRef, all []stepRef, root *yaml.Node, content string, fileLines []string, spans map[int]lineSpan) envPlan {
	var p envPlan
	uses := mappingValue(s.step, "uses")
	script := mappingValue(mappingValue(s.step, "with"), "script")
	env := mappingValue(s.step, "env")
	if uses == nil || !scriptAction.MatchString(uses.Value) || script == nil || script.Kind != yaml.ScalarNode {
		return p
	}
	if env == nil || env.Kind != yaml.MappingNode || env.Style&yaml.FlowStyle != 0 || wholeEnv(script.Value) {
		return p
	}
	at, ok := scalarLines(script, fileLines, spans)
	if !ok {
		return p
	}
	others := stepTextOutsideScript(s.step, script) + stepsText(all, s.step)
	strs := stringInputs(root)
	var with *yaml.Node
	if typescriptAction.MatchString(uses.Value) {
		with = mappingValue(s.step, "with")
	}
	var swapped []lineSwap
	drop := set.New[int]()
	roots := set.New[string]()
	kept := 0
	for i := 0; i+1 < len(env.Content); i += 2 {
		key, value := env.Content[i], env.Content[i+1]
		read, swaps, ok := judgeScriptEntry(key, value, script.Value, at, others, strs, with)
		if !ok {
			kept++
			continue
		}
		roots.Add(read.root)
		p.findings = append(p.findings, ste.Finding{
			Line: key.Line, ID: IDEnvIndirection,
			Rule:   "an env entry only carries an expression into the script",
			Detail: key.Value + ": " + value.Value,
			Fix:    "Write " + read.text + " where the script reads process.env." + key.Value + ", and remove the entry.",
		})
		for _, sw := range swaps {
			sw.literal = read.text
			swapped = append(swapped, sw)
		}
		drop.Add(key.Line - 1)
		for n := range commentLines(key) {
			above := key.Line - 2 - n
			if above < 0 || !strings.HasPrefix(strings.TrimSpace(fileLines[above]), "#") {
				break
			}
			drop.Add(above)
		}
	}
	if len(p.findings) == 0 {
		return p
	}
	if envKey := mappingKey(s.step, "env"); kept == 0 && envKey != nil && envKey.Line != env.Content[0].Line {
		drop.Add(envKey.Line - 1)
	}
	rows := map[int][]string{}
	for k := range at.file {
		if after := applySwaps(at.file[k], at.prefix[k], k, swapped); after != at.file[k] {
			rows[at.lineNos[k]] = []string{after}
		}
	}
	roots.Remove("")
	if roots.Len() > 0 {
		entries, row := withEntries(with, roots, fileLines)
		addEntries(rows, row, entries, fileLines)
	}
	p.edits = append(p.edits, rowEdits(content, rows)...)
	p.edits = append(p.edits, dropLines(content, drop)...)
	return p
}

// applySwaps writes each swap's literal over its reads on script line k. A
// read inside the dead arm of an earlier read goes with that arm.
func applySwaps(fileLine, prefix string, k int, swaps []lineSwap) string {
	type span struct {
		start, end int
		text       string
	}
	var spans []span
	for _, sw := range swaps {
		if sw.line != k {
			continue
		}
		for _, r := range sw.reads {
			spans = append(spans, span{r.start, r.end, sw.literal})
		}
	}
	if len(spans) == 0 {
		return fileLine
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	script := fileLine[len(prefix):]
	var b strings.Builder
	b.WriteString(prefix)
	at := 0
	for _, s := range spans {
		if s.start < at {
			continue
		}
		b.WriteString(script[at:s.start])
		b.WriteString(s.text)
		at = s.end
	}
	b.WriteString(script[at:])
	return b.String()
}

// commentLines answers how many comment lines sit above a key as its head comment.
func commentLines(key *yaml.Node) int {
	if key.HeadComment == "" {
		return 0
	}
	return strings.Count(key.HeadComment, "\n") + 1
}

// lineSwap is the reads of one variable on one script line, and the literal
// that takes their place.
type lineSwap struct {
	line    int
	reads   []envRead
	literal string
}

// judgeScriptEntry answers the literal for an entry the script reads only as
// process.env.NAME, and where each read sits. It answers false for an entry
// with a job to do. A value that is not one expression, a secret, a read the
// literal cannot stand in for, or a mention anywhere else.
func judgeScriptEntry(key, value *yaml.Node, script string, at placedLines, others string, strs set.Set[string], with *yaml.Node) (entryRead, []lineSwap, bool) {
	name := key.Value
	if value.Kind != yaml.ScalarNode || value.Line != key.Line || value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return entryRead{}, nil, false
	}
	match := wholeExpression.FindStringSubmatch(strings.TrimSpace(value.Value))
	if match == nil || secret.MatchString(match[1]) || strings.Contains(match[1], "\n") {
		return entryRead{}, nil, false
	}
	if mentions(others, name) > 0 || shellReads(others, name) > 0 {
		return entryRead{}, nil, false
	}
	var swaps []lineSwap
	total := 0
	for k := range at.file {
		reads, ok := exactReads(at.file[k][len(at.prefix[k]):], name)
		if !ok {
			return entryRead{}, nil, false
		}
		if len(reads) > 0 {
			swaps = append(swaps, lineSwap{line: k, reads: reads})
			total += len(reads)
		}
	}
	if total == 0 || mentions(script, name) != total {
		return entryRead{}, nil, false
	}
	text, root := readOf(match[1], strs, with)
	return entryRead{text: text, root: root}, swaps, true
}

// entryRead is what the script reads in place of an env entry, and the context
// the step must pass for it, or "".
type entryRead struct {
	text, root string
}

// stepTextOutsideScript answers every scalar of the step except its script,
// so an env.NAME in an if or another with input keeps the entry.
func stepTextOutsideScript(step, script *yaml.Node) string {
	env := mappingValue(step, "env")
	var out strings.Builder
	walk(step, func(n *yaml.Node) {
		if n == script || n.Kind != yaml.ScalarNode {
			return
		}
		if env != nil && isEnvKey(env, n) {
			return
		}
		out.WriteString(n.Value)
		out.WriteByte('\n')
	})
	return out.String()
}

// isEnvKey reports whether n is a key of the env mapping.
func isEnvKey(env, n *yaml.Node) bool {
	for i := 0; i+1 < len(env.Content); i += 2 {
		if env.Content[i] == n {
			return true
		}
	}
	return false
}
