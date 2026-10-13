package workflow

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// publicOnly is the condition that keeps a job or a matrix value to a public repository.
const publicOnly = "github.event.repository.visibility == 'public'"

// visibilityTest matches a condition that already reads the repository's visibility.
var visibilityTest = regexp.MustCompile(`github\.event\.repository\.visibility\s*==\s*'public'`)

// hostedForeign matches, in any case, a GitHub-hosted macOS, Windows or arm label.
var hostedForeign = regexp.MustCompile(`(?i)^(macos|windows)-[A-Za-z0-9.-]+$|^ubuntu-(latest|[0-9]+\.[0-9]+)-arm$`)

// matrixKeyRef matches each matrix value an expression reads.
var matrixKeyRef = regexp.MustCompile(`\bmatrix\.([A-Za-z_][A-Za-z0-9_-]*)`)

// routedReference matches the runs-on expression that routes the hosted Linux values of a matrix.
var routedReference = regexp.MustCompile(`^\$\{\{ \(github\.event\.repository\.visibility == 'public' \|\| !contains\(fromJSON\('[^']*'\), matrix\.([A-Za-z_][A-Za-z0-9_-]*)\)\) && matrix\.[A-Za-z0-9_-]+ \|\| vars\.CI_RUNNER(_DIND)? \|\| matrix\.[A-Za-z0-9_-]+ \}\}$`)

// matrixLabel matches a matrix value the repair can write into a JSON list.
var matrixLabel = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// foreignJob is a job that runs on a hosted label the org runners cannot serve.
type foreignJob struct {
	on *yaml.Node
	// onKey is the runs-on key. A new if: goes on the line above it.
	onKey *yaml.Node
	// cond is the job's if: value, or nil.
	cond *yaml.Node
	// listKey and list are the matrix key and its values when only some values are foreign.
	listKey *yaml.Node
	list    *yaml.Node
	// listFlow reports whether the list sits inside a flow mapping.
	listFlow bool
	// includeKey and include are the matrix include list when an entry of it is foreign.
	includeKey *yaml.Node
	include    *yaml.Node
	// key is the matrix key runs-on reads.
	key     string
	foreign []string
}

// foreignJobs answers each job of content on a foreign hosted label that the
// repair can keep to a public repository.
func foreignJobs(content string) []foreignJob {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(content), &doc) != nil {
		return nil
	}
	root := rootOf(&doc)
	jobs := mappingValue(root, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode || (root.Style|jobs.Style)&yaml.FlowStyle != 0 {
		return nil
	}
	var out []foreignJob
	for i := 1; i < len(jobs.Content); i += 2 {
		job := jobs.Content[i]
		if job.Kind != yaml.MappingNode || job.Style&yaml.FlowStyle != 0 {
			continue
		}
		f, ok := foreignJobOf(job)
		if ok {
			out = append(out, f)
		}
	}
	return out
}

// foreignJobOf reads one block-style job.
func foreignJobOf(job *yaml.Node) (foreignJob, bool) {
	var f foreignJob
	for i := 0; i+1 < len(job.Content); i += 2 {
		switch job.Content[i].Value {
		case "runs-on":
			f.onKey, f.on = job.Content[i], job.Content[i+1]
		case "if":
			f.cond = job.Content[i+1]
		}
	}
	if f.on == nil || f.on.Kind != yaml.ScalarNode {
		return f, false
	}
	if f.cond != nil && (f.cond.Kind != yaml.ScalarNode || visibilityTest.MatchString(f.cond.Value) || !conditionReadable(f.cond.Value)) {
		return f, false
	}
	if hostedForeign.MatchString(f.on.Value) {
		f.foreign = []string{f.on.Value}
		return f, true
	}
	m := matrixReference.FindStringSubmatch(f.on.Value)
	if m == nil {
		m = routedReference.FindStringSubmatch(f.on.Value)
	}
	if m != nil {
		return foreignMatrix(f, job, m[1])
	}
	key, ok := onlyMatrixKey(f.on.Value)
	if !ok {
		return f, false
	}
	return foreignMatrix(f, job, key)
}

// onlyMatrixKey answers the one matrix key that a runs-on expression reads,
// such as os in ${{ startsWith(matrix.os, 'ubuntu') && vars.CI_RUNNER || matrix.os }}.
// The repair drops the foreign values from that key, and leaves the expression as written.
func onlyMatrixKey(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "${{") || !strings.HasSuffix(value, "}}") || strings.Count(value, "${{") != 1 {
		return "", false
	}
	refs := matrixKeyRef.FindAllStringSubmatch(value, -1)
	if len(refs) == 0 {
		return "", false
	}
	for _, ref := range refs {
		if ref[1] != refs[0][1] {
			return "", false
		}
	}
	return refs[0][1], true
}

// foreignMatrix reads the values a matrix gives key. Every value foreign: the
// job-level if: keeps the job to a public repository. Some values foreign: a
// list under the key, and an include list that names a foreign value, each get
// a visibility expression. The rule reads no other shape.
func foreignMatrix(f foreignJob, job *yaml.Node, key string) (foreignJob, bool) {
	strategy := mappingValue(job, "strategy")
	matrix := mappingValue(strategy, "matrix")
	if matrix == nil || matrix.Kind != yaml.MappingNode {
		return f, false
	}
	var list, includeKey, include *yaml.Node
	for i := 0; i+1 < len(matrix.Content); i += 2 {
		switch matrix.Content[i].Value {
		case key:
			f.listKey, list = matrix.Content[i], matrix.Content[i+1]
		case "include":
			includeKey, include = matrix.Content[i], matrix.Content[i+1]
		}
	}
	keptIncluded, foreignIncluded := 0, false
	if include != nil {
		if include.Kind != yaml.SequenceNode {
			return f, false
		}
		for _, entry := range include.Content {
			v := mappingValue(entry, key)
			if v == nil {
				continue
			}
			if entry.Kind != yaml.MappingNode || v.Kind != yaml.ScalarNode {
				return f, false
			}
			if hostedForeign.MatchString(v.Value) {
				foreignIncluded = true
				f.foreign = append(f.foreign, v.Value)
			} else {
				keptIncluded++
			}
		}
	}
	kept := 0
	if list != nil {
		if list.Kind != yaml.SequenceNode || list.Anchor != "" || len(list.Content) == 0 {
			return f, false
		}
		for _, v := range list.Content {
			if v.Kind != yaml.ScalarNode || !matrixLabel.MatchString(v.Value) {
				return f, false
			}
			if hostedForeign.MatchString(v.Value) {
				f.foreign = append(f.foreign, v.Value)
			} else {
				kept++
			}
		}
	}
	if len(f.foreign) == 0 {
		return f, false
	}
	if kept == 0 && keptIncluded == 0 {
		// No leg is left for a private repository, so the job-level if: keeps the whole job to a public one.
		f.listKey = nil
		return f, true
	}
	if list != nil && kept == 0 {
		// An empty list beside an include entry is a matrix GitHub refuses.
		return f, false
	}
	f.listFlow = (strategy.Style|matrix.Style)&yaml.FlowStyle != 0
	if list != nil && len(list.Content) > kept {
		if f.listFlow && list.Style&yaml.FlowStyle == 0 {
			return f, false
		}
		f.list = list
	} else {
		f.listKey = nil
	}
	if foreignIncluded {
		if f.listFlow || include.Style&yaml.FlowStyle != 0 || include.Anchor != "" {
			return f, false
		}
		if _, _, ok := includeLists(include, key); !ok {
			return f, false
		}
		f.includeKey, f.include, f.key = includeKey, include, key
	}
	return f, true
}

// includeLists answers the include list as JSON, whole and without the
// entries that name a foreign value of key.
func includeLists(include *yaml.Node, key string) (whole, private string, ok bool) {
	var all, kept []string
	for _, entry := range include.Content {
		text, ok := jsonOf(entry)
		if !ok {
			return "", "", false
		}
		all = append(all, text)
		if v := mappingValue(entry, key); v == nil || !hostedForeign.MatchString(v.Value) {
			kept = append(kept, text)
		}
	}
	whole = "[" + strings.Join(all, ",") + "]"
	private = "[" + strings.Join(kept, ",") + "]"
	// The lists sit in an expression string, which "}}" or "${{" would end or open.
	if strings.Contains(whole, "}}") || strings.Contains(whole, "${{") {
		return "", "", false
	}
	return whole, private, true
}

// jsonOf answers node as JSON with the value GitHub reads from it. It refuses
// an alias, a merge key and a multi-line scalar. It also refuses a scalar
// that JSON spells another way.
func jsonOf(node *yaml.Node) (string, bool) {
	switch node.Kind {
	case yaml.ScalarNode:
		if strings.Contains(node.Value, "\n") {
			return "", false
		}
		switch node.ShortTag() {
		case "!!str":
			text, _ := json.Marshal(node.Value)
			return string(text), true
		case "!!int", "!!float", "!!bool", "!!null":
			return node.Value, json.Valid([]byte(node.Value))
		}
	case yaml.SequenceNode:
		var items []string
		for _, item := range node.Content {
			text, ok := jsonOf(item)
			if !ok {
				return "", false
			}
			items = append(items, text)
		}
		return "[" + strings.Join(items, ",") + "]", node.Anchor == ""
	case yaml.MappingNode:
		var pairs []string
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i]
			if k.Kind != yaml.ScalarNode || k.ShortTag() != "!!str" {
				return "", false
			}
			value, ok := jsonOf(node.Content[i+1])
			if !ok {
				return "", false
			}
			name, _ := json.Marshal(k.Value)
			pairs = append(pairs, string(name)+":"+value)
		}
		body := strings.Join(pairs, ",")
		if strings.HasSuffix(body, "}") {
			body += " "
		}
		return "{" + body + "}", node.Anchor == ""
	}
	return "", false
}

// conditionReadable reports whether the repair can AND a condition: an
// expression, or one ${{ }} that holds the whole value.
func conditionReadable(value string) bool {
	value = strings.TrimSpace(value)
	count := strings.Count(value, "${{")
	return count == 0 || (count == 1 && strings.HasPrefix(value, "${{") && strings.HasSuffix(value, "}}"))
}

// gatedCondition answers the if: value that keeps a job to a public repository.
func gatedCondition(cond *yaml.Node) string {
	if cond == nil {
		return publicOnly
	}
	value := strings.TrimSpace(cond.Value)
	if strings.HasPrefix(value, "${{") {
		inner := strings.TrimSpace(value[3 : len(value)-2])
		return "${{ (" + inner + ") && " + publicOnly + " }}"
	}
	return "(" + value + ") && " + publicOnly
}

// splitList answers the matrix list expression that gives a private repository only the values the org runners serve.
func splitList(list *yaml.Node) string {
	var all, kept []string
	for _, v := range list.Content {
		all = append(all, v.Value)
		if !hostedForeign.MatchString(v.Value) {
			kept = append(kept, v.Value)
		}
	}
	whole, _ := json.Marshal(all)
	private, _ := json.Marshal(kept)
	return "${{ " + publicOnly + " && fromJSON('" + string(whole) + "') || fromJSON('" + string(private) + "') }}"
}

// yamlValue answers value as a YAML scalar: plain where that reads back the
// same, double-quoted elsewhere.
func yamlValue(value string, flow bool) string {
	plain := !flow && !strings.ContainsAny(value, "\n\t") && !strings.Contains(value, ": ") &&
		!strings.Contains(value, " #") && !strings.HasSuffix(value, ":")
	if plain {
		return value
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`)
	return `"` + r.Replace(value) + `"`
}

// foreignFinding reports one job that runs on a foreign hosted label.
func foreignFinding(repo string, f foreignJob) ste.Finding {
	fix := "Add `if: " + publicOnly + "` to the job, or AND it into the existing if:."
	var writes []string
	if f.list != nil {
		writes = append(writes, "`"+f.listKey.Value+": "+splitList(f.list)+"`")
	}
	if f.include != nil {
		writes = append(writes, "`include: "+includeSplit(f)+"`")
	}
	if len(writes) > 0 {
		fix = "Write " + strings.Join(writes, " and ") + "."
	}
	return ste.Finding{
		Line:   f.on.Line,
		ID:     IDPrivateRunner,
		Rule:   "an org job on a hosted runner that the org runners cannot replace",
		Detail: repo + ": runs-on: " + f.on.Value + " (" + strings.Join(f.foreign, ", ") + ")",
		Fix:    fix + " The org runners are Linux x64, so a private repository skips " + strings.Join(f.foreign, ", ") + ". A public repository keeps it. `slopfix fix` does this.",
	}
}

// foreignEdits writes the public-only condition or the split list into one job.
func foreignEdits(content string, fileLines []string, f foreignJob) []edit.Edit {
	if f.list != nil || f.include != nil {
		var out []edit.Edit
		if f.list != nil {
			e, ok := splitEdit(content, fileLines, f)
			if !ok {
				return nil
			}
			out = append(out, e)
		}
		if f.include != nil {
			e, ok := includeEdit(content, fileLines, f)
			if !ok {
				return nil
			}
			out = append(out, e)
		}
		return out
	}
	if f.cond != nil {
		first, last, before, after, ok := scalarSource(content, fileLines, f.cond, false)
		if !ok {
			return nil
		}
		return []edit.Edit{rewrite(content, first, last, []string{before + yamlValue(gatedCondition(f.cond), false) + after})}
	}
	line, col := f.onKey.Line-1, f.onKey.Column-1
	if line < 0 || line >= len(fileLines) || col > len(fileLines[line]) || strings.TrimSpace(fileLines[line][:col]) != "" {
		return nil
	}
	indent := fileLines[line][:col]
	return []edit.Edit{rewrite(content, line, line, []string{indent + "if: " + publicOnly, fileLines[line]})}
}

// includeSplit answers the include expression that gives a private repository
// only the entries whose legs the org runners serve.
func includeSplit(f foreignJob) string {
	whole, private, _ := includeLists(f.include, f.key)
	quote := strings.NewReplacer("'", "''")
	return "${{ " + publicOnly + " && fromJSON('" + quote.Replace(whole) + "') || fromJSON('" + quote.Replace(private) + "') }}"
}

// includeEdit replaces a block include list with its visibility expression on
// one line under the key. The next line must sit at or left of the key, so no
// continuation of the last entry is left behind.
func includeEdit(content string, fileLines []string, f foreignJob) (edit.Edit, bool) {
	keyLine := f.includeKey.Line - 1
	last := deepestLine(f.include) - 1
	if keyLine < 0 || last < keyLine || last >= len(fileLines) || f.includeKey.Column-1 > len(fileLines[keyLine]) {
		return edit.Edit{}, false
	}
	for _, next := range fileLines[last+1:] {
		trimmed := strings.TrimSpace(next)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(next)-len(strings.TrimLeft(next, " \t")) > f.includeKey.Column-1 {
			return edit.Edit{}, false
		}
		break
	}
	colon := strings.Index(fileLines[keyLine][f.includeKey.Column-1:], ":")
	if colon < 0 {
		return edit.Edit{}, false
	}
	head := fileLines[keyLine][:f.includeKey.Column-1+colon+1]
	return rewrite(content, keyLine, last, []string{head + " " + yamlValue(includeSplit(f), false)}), true
}

// deepestLine answers the last 1-based line a node or a descendant starts on.
func deepestLine(node *yaml.Node) int {
	line := node.Line
	for _, child := range node.Content {
		line = max(line, deepestLine(child))
	}
	return line
}

// splitEdit replaces the matrix list with the visibility expression. A flow
// list keeps the text around it. A block list becomes one line under its key.
func splitEdit(content string, fileLines []string, f foreignJob) (edit.Edit, bool) {
	list := f.list
	lastItem := list.Content[len(list.Content)-1]
	if list.Style&yaml.FlowStyle != 0 {
		first, last := list.Line-1, lastItem.Line-1
		if first < 0 || last >= len(fileLines) || list.Column-1 > len(fileLines[first]) {
			return edit.Edit{}, false
		}
		from := lastItem.Column - 1
		if from < 0 || from > len(fileLines[last]) {
			return edit.Edit{}, false
		}
		end := strings.IndexByte(fileLines[last][from:], ']')
		if end < 0 {
			return edit.Edit{}, false
		}
		value := yamlValue(splitList(list), f.listFlow)
		text := fileLines[first][:list.Column-1] + value + fileLines[last][from+end+1:]
		return rewrite(content, first, last, []string{text}), true
	}
	keyLine := f.listKey.Line - 1
	last := lastItem.Line - 1
	for _, v := range list.Content {
		if v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			return edit.Edit{}, false
		}
	}
	if keyLine < 0 || last < keyLine || last >= len(fileLines) || f.listKey.Column-1 > len(fileLines[keyLine]) {
		return edit.Edit{}, false
	}
	colon := strings.Index(fileLines[keyLine][f.listKey.Column-1:], ":")
	if colon < 0 {
		return edit.Edit{}, false
	}
	head := fileLines[keyLine][:f.listKey.Column-1+colon+1]
	return rewrite(content, keyLine, last, []string{head + " " + yamlValue(splitList(list), false)}), true
}
