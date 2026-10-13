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

// hostedForeign matches a GitHub-hosted label the org runners cannot serve: macOS, Windows or arm.
var hostedForeign = regexp.MustCompile(`^(macos|windows)-[A-Za-z0-9.-]+$|^ubuntu-(latest|[0-9]+\.[0-9]+)-arm$`)

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
	foreign  []string
}

// foreignJobs answers each job of content that runs on a foreign hosted label
// in a private repository, and that the repair can keep to a public one.
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
	if m == nil {
		return f, false
	}
	return foreignMatrix(f, job, m[1])
}

// foreignMatrix reads the values a matrix gives key. Every value foreign: the
// job-level if: keeps the job to a public repository. Some values foreign: a
// list under the key gets a visibility expression. The rule reads no other shape.
func foreignMatrix(f foreignJob, job *yaml.Node, key string) (foreignJob, bool) {
	strategy := mappingValue(job, "strategy")
	matrix := mappingValue(strategy, "matrix")
	if matrix == nil || matrix.Kind != yaml.MappingNode {
		return f, false
	}
	included := false
	if include := mappingValue(matrix, "include"); include != nil {
		if include.Kind != yaml.SequenceNode {
			return f, false
		}
		for _, entry := range include.Content {
			v := mappingValue(entry, key)
			if v == nil {
				continue
			}
			// An include entry that names a dropped value would add that value back as a new leg.
			if entry.Kind != yaml.MappingNode || v.Kind != yaml.ScalarNode || hostedForeign.MatchString(v.Value) {
				return f, false
			}
			included = true
		}
	}
	var list *yaml.Node
	for i := 0; i+1 < len(matrix.Content); i += 2 {
		if matrix.Content[i].Value == key {
			f.listKey, list = matrix.Content[i], matrix.Content[i+1]
		}
	}
	if list == nil || list.Kind != yaml.SequenceNode || list.Anchor != "" || len(list.Content) == 0 {
		return f, false
	}
	kept := 0
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
	if len(f.foreign) == 0 {
		return f, false
	}
	if kept == 0 {
		// An empty list beside an include entry is a matrix GitHub refuses.
		f.listKey = nil
		return f, !included
	}
	f.listFlow = (strategy.Style|matrix.Style)&yaml.FlowStyle != 0
	if f.listFlow && list.Style&yaml.FlowStyle == 0 {
		return f, false
	}
	f.list = list
	return f, true
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
	if f.list != nil {
		fix = "Write `" + f.listKey.Value + ": " + splitList(f.list) + "`."
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
	if f.list != nil {
		e, ok := splitEdit(content, fileLines, f)
		if !ok {
			return nil
		}
		return []edit.Edit{e}
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
