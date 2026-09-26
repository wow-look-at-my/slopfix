// repair.go rewrites what a workflow rule finds, wherever a rewrite says the
// same thing the author meant.
//
// A newline is syntax in YAML, so every repair here works on whole lines and
// never reflows. What no rewrite can say is left to the author, and the report
// keeps naming it.
package workflow

import (
	"reflect"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// Repair is a workflow as this binary would write it.
type Repair struct {
	// Text is the repaired content. It equals the input when Changed is false.
	Text string
	// Changed reports whether any rewrite applied.
	Changed bool
	// Removed names each line the repair cut.
	Removed []string
}

// Fix applies every repair the caller keeps. keeps takes a rule ID, so a run
// that named a rule gets that rule alone.
func Fix(content string, keeps func(string) bool) Repair {
	res := FixIn("", content, keeps, edit.Scope{})
	return Repair{Text: res.Text, Changed: res.Text != content, Removed: res.Cuts()}
}

// FixIn is Fix with every edit held inside scope. Each pass goes through the
// YAML gate, and a comment edit must leave the document's data as it was.
func FixIn(path, content string, keeps func(string) bool, scope edit.Scope) edit.Result {
	out := edit.Unchanged(content, scope)
	pass := func(edits func(string) []edit.Edit, comment bool) {
		out = out.Then(apply(out.Text, edits(out.Text), out.Scope, comment))
	}
	if keeps(IDNeuteredGate) {
		pass(ungate, false)
	}
	if keeps(IDCommentBlock) {
		pass(joinCommentBlocks, true)
	}
	if keeps(IDAllBuildsJob) {
		pass(renameGuardedJob, false)
	}
	if keeps(IDTestInYAML) {
		pass(untest, false)
	}
	return out
}

// apply writes edits into a workflow through the YAML gate. Every edit covers
// whole rows, because a newline is syntax here. The result must parse, and a
// comment edit must decode to the same data.
func apply(content string, edits []edit.Edit, scope edit.Scope, comment bool) edit.Result {
	if len(edits) == 0 {
		return edit.Unchanged(content, scope)
	}
	var want any
	if yaml.Unmarshal([]byte(content), &want) != nil {
		out := edit.Unchanged(content, scope)
		for _, e := range edits {
			out.Refused = append(out.Refused, edit.Refused{Edit: e, Reason: "the workflow does not parse"})
		}
		return out
	}
	return edit.Gate(content, edits, scope,
		func(e edit.Edit) string {
			if !rowEdge(content, e.Start) || !rowEdge(content, e.End) {
				return "it covers part of a row"
			}
			return ""
		},
		func(text string) bool {
			var got any
			if yaml.Unmarshal([]byte(text), &got) != nil {
				return false
			}
			return !comment || reflect.DeepEqual(got, want)
		})
}

// rowEdge reports whether a byte sits where a row starts or ends.
func rowEdge(content string, at int) bool {
	return at == 0 || at == len(content) || content[at] == '\n' || content[at-1] == '\n'
}

// rewrite replaces rows from..to with lines, keeping a carriage return the
// last row carried, because lines reads rows without it.
func rewrite(content string, from, to int, lines []string) edit.Edit {
	e := edit.Rows(content, from, to, 0, lines)
	if e.End > 0 && content[e.End-1] == '\r' {
		e.Text += "\r"
	}
	return e
}

// dropRows deletes the marked rows, a run of adjoining rows as a single edit,
// each quoting what it takes.
func dropRows(content string, drop set.Set[int]) []edit.Edit {
	rows := lines(content)
	var out []edit.Edit
	for row := 0; row < len(rows); row++ {
		if !drop.Contains(row) {
			continue
		}
		end := row
		for end+1 < len(rows) && drop.Contains(end+1) {
			end++
		}
		e := edit.Rows(content, row, end, 0, nil)
		for r := row; r <= end; r++ {
			e.Cut = append(e.Cut, strings.TrimSpace(rows[r]))
		}
		out = append(out, e)
		row = end
	}
	return out
}

// ungate deletes the continue-on-error a gate step hides behind. The step stays
// and starts failing.
func ungate(content string) []edit.Edit {
	findings := neuteredGates(content)
	if len(findings) == 0 {
		return nil
	}
	return dropRows(content, gateRows(content, findings))
}

// gateRows answers the row each named step's continue-on-error sits on, read
// off the parser's own positions. Walking the text for the step's extent
// instead asks an indent to say where a step ends, and a block scalar holding
// a deeper line then ends it early.
func gateRows(content string, findings []ste.Finding) set.Set[int] {
	drop := set.New[int]()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return drop
	}
	jobs := mappingValue(rootOf(&doc), "jobs")
	if jobs == nil {
		return drop
	}
	named := set.New[int]()
	for _, f := range findings {
		named.Add(f.Line)
	}
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		steps := mappingValue(jobs.Content[i+1], "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			if !named.Contains(step.Line) {
				continue
			}
			if key := mappingKey(step, allowedToFailKey); key != nil {
				drop.Add(key.Line - 1)
			}
		}
	}
	return drop
}

// allowedToFailKey is the key a gate hides behind.
const allowedToFailKey = "continue-on-error"

// mappingKey answers the key node itself, where mappingValue answers what it
// carries. A repair needs the key's own line, because the key is the line.
func mappingKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i]
		}
	}
	return nil
}

// untest deletes the assertion lines a run: script carries, and the caller
// prints each: the suite is where a case belongs and this file is not it. A
// step emptied of them keeps its shape, because removing it is the author's call.
func untest(content string) []edit.Edit {
	findings := testsInYAML(content)
	if len(findings) == 0 {
		return nil
	}
	rows := lines(content)
	drop := set.New[int]()
	for _, f := range findings {
		if f.Line-1 < len(rows) {
			drop.Add(f.Line - 1)
		}
	}
	return dropRows(content, drop)
}

// joinCommentBlocks folds a run of comment lines into the line the rule
// allows, keeping every word. The gate proves the fold changed no data.
func joinCommentBlocks(content string) []edit.Edit {
	findings := commentBlocks(content)
	if len(findings) == 0 {
		return nil
	}
	rows := lines(content)
	var out []edit.Edit
	for _, f := range findings {
		first, last := f.Line-1, min(f.EndLine, len(rows))-1
		if first < 0 || last < first {
			continue
		}
		var said, cut, rest []string
		for j := first; j <= last; j++ {
			trimmed := strings.TrimSpace(rows[j])
			if !strings.HasPrefix(trimmed, "#") {
				// A blank row inside the run is no comment, and it stays.
				rest = append(rest, rows[j])
				continue
			}
			if words := strings.TrimSpace(strings.TrimPrefix(trimmed, "#")); words != "" {
				said = append(said, words)
			}
			if j > first {
				cut = append(cut, trimmed)
			}
		}
		indent := rows[first][:len(rows[first])-len(strings.TrimLeft(rows[first], " \t"))]
		joined := strings.TrimRight(indent+"# "+strings.Join(said, " "), " ")
		e := rewrite(content, first, last, append([]string{joined}, rest...))
		e.Cut = cut
		out = append(out, e)
	}
	return out
}

// renameGuardedJob renames a job that shadows the required status, and the
// needs entries that point at it. The name is the whole finding, so renaming it
// is the whole repair.
func renameGuardedJob(content string) []edit.Edit {
	rows := lines(content)
	var out []edit.Edit
	for _, at := range guardedSites(content) {
		row := at.Line - 1
		if row < 0 || row >= len(rows) {
			continue
		}
		swapped, ok := renameAt(rows[row], at.Column-1)
		if !ok {
			continue
		}
		out = append(out, rewrite(content, row, row, []string{swapped}))
	}
	return out
}

// guardedSites answers every token naming the guarded job: the job's own key,
// and each needs entry pointing at it.
func guardedSites(content string) []*yaml.Node {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	jobs := mappingValue(rootOf(&doc), "jobs")
	if jobs == nil {
		return nil
	}
	var out []*yaml.Node
	for i := 0; i+1 < len(jobs.Content); i += 2 {
		key, job := jobs.Content[i], jobs.Content[i+1]
		if key.Value == GuardedName {
			out = append(out, key)
		}
		out = append(out, guardedNeeds(job)...)
	}
	return out
}

// guardedNeeds answers the needs entries of a single job that name the
// guarded job. A single dependency is a scalar, and several are a sequence.
func guardedNeeds(job *yaml.Node) []*yaml.Node {
	needs := mappingValue(job, "needs")
	if needs == nil {
		return nil
	}
	if needs.Kind == yaml.ScalarNode {
		if needs.Value == GuardedName {
			return []*yaml.Node{needs}
		}
		return nil
	}
	var out []*yaml.Node
	for _, entry := range needs.Content {
		if entry.Kind == yaml.ScalarNode && entry.Value == GuardedName {
			out = append(out, entry)
		}
	}
	return out
}

// renameAt swaps the guarded name for its replacement at a column the parser
// gave, and reports whether the name was there.
func renameAt(row string, col int) (string, bool) {
	for _, at := range []int{col, col + 1} {
		if at < 0 || at+len(GuardedName) > len(row) {
			continue
		}
		if row[at:at+len(GuardedName)] != GuardedName {
			continue
		}
		return row[:at] + replacementName + row[at+len(GuardedName):], true
	}
	return row, false
}

// replacementName is a job name the gate does not reserve.
const replacementName = "builds"

