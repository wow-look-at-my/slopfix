package workflow

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/forkscope"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// IDPrivateRunner is a job of an org repository whose runs-on ignores the repository's visibility.
const IDPrivateRunner = "yaml/private-runner"

// The org variables that name the org's own runners.
const (
	runnerVar     = "vars.CI_RUNNER"
	runnerDindVar = "vars.CI_RUNNER_DIND"
)

// SelfHostExempt names each org repository the self-hosted fleet depends on.
var SelfHostExempt = set.Of(
	"wow-look-at-my/webhook-runner",
	"wow-look-at-my/webhooks",
	"wow-look-at-my/buildhost",
	"wow-look-at-my/github-state-mirror",
)

// hostedLinux matches a GitHub-hosted x64 Linux label.
var hostedLinux = regexp.MustCompile(`^ubuntu-(latest|[0-9]+\.[0-9]+)$`)

// blindFallback matches an expression that picks an org runner variable.
var blindFallback = regexp.MustCompile(`^\$\{\{\s*vars\.(CI_RUNNER|CI_RUNNER_DIND)\s*\|\|\s*'(ubuntu-(?:latest|[0-9]+\.[0-9]+))'\s*\}\}$`)

// dockerCommand matches a script line that runs the docker client.
var dockerCommand = regexp.MustCompile(`(^|[\s;&|(])(sudo\s+)?docker(-compose)?(\s|$)`)

// dockerAction matches a uses: value whose action runs the docker client. The org's ghcr action builds with buildx.
var dockerAction = regexp.MustCompile(`docker|buildx|^wow-look-at-my/actions@ghcr#`)

// firstParty matches an action owner whose actions the rule knows. Their docker use is in dockerAction.
var firstParty = regexp.MustCompile(`^(actions|wow-look-at-my)/`)

// governedRepo answers the org repository the file at path belongs to, a
// directory of its work tree, and whether the rule reads it. The name comes
// from the work tree alone.
func governedRepo(path string) (string, string, bool) {
	if path == "" {
		return "", "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", false
	}
	dir := forkscope.ExistingDir(filepath.Dir(abs))
	repo := forkscope.Resolver{}.RepoName(dir)
	owner, _, _ := strings.Cut(repo, "/")
	if !strings.EqualFold(owner, orgOwner) || SelfHostExempt.Contains(strings.ToLower(repo)) {
		return "", "", false
	}
	return repo, dir, true
}

// hostedJob is a job whose runs-on ignores the repository's visibility.
type hostedJob struct {
	job   *yaml.Node
	node  *yaml.Node
	label string
	// key is the matrix key runs-on reads, with its hosted Linux values in hosted.
	key    string
	hosted []string
	docker bool
	// flow reports whether the runs-on value sits inside a flow collection.
	flow bool
	// unread names each action whose runs.using could not be read. The job then goes to the docker runner.
	unread []string
}

// governedJobs answers each job of the file at path that the rule reports,
// with its docker need, and the repository.
func governedJobs(path, content string) ([]hostedJob, string) {
	jobs := hostedJobs(content)
	if len(jobs) == 0 {
		return nil, ""
	}
	repo, dir, governed := governedRepo(path)
	if !governed {
		return nil, ""
	}
	for idx := range jobs {
		docker, unread := runsDocker(jobs[idx].job, dir)
		jobs[idx].docker = jobs[idx].docker || docker
		jobs[idx].unread = unread
	}
	return jobs, repo
}

// runnerLabel answers the hosted Linux label of a runs-on value. It also
// answers whether the value names the docker runner variable.
func runnerLabel(value string) (label string, dind bool) {
	if hostedLinux.MatchString(value) {
		return value, false
	}
	if m := blindFallback.FindStringSubmatch(value); m != nil {
		return m[2], m[1] == "CI_RUNNER_DIND"
	}
	return "", false
}

// hostedJobs answers each job of content whose runs-on ignores the
// visibility, in source order.
func hostedJobs(content string) []hostedJob {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(content), &doc) != nil {
		return nil
	}
	root := rootOf(&doc)
	jobs := mappingValue(root, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return nil
	}
	var out []hostedJob
	for i := 1; i < len(jobs.Content); i += 2 {
		job := jobs.Content[i]
		on := mappingValue(job, "runs-on")
		if on == nil || on.Kind != yaml.ScalarNode {
			continue
		}
		flow := (root.Style|jobs.Style|job.Style)&yaml.FlowStyle != 0
		label, dind := runnerLabel(on.Value)
		if label != "" {
			out = append(out, hostedJob{job: job, node: on, label: label, docker: dind, flow: flow})
			continue
		}
		if key, hosted := matrixRunner(job, on.Value); len(hosted) > 0 {
			out = append(out, hostedJob{job: job, node: on, key: key, hosted: hosted, flow: flow})
		}
	}
	return out
}

// matrixReference matches a runs-on that is one matrix value, such as ${{ matrix.os }}.
var matrixReference = regexp.MustCompile(`^\$\{\{\s*matrix\.([A-Za-z_][A-Za-z0-9_-]*)\s*\}\}$`)

// matrixRunner answers the matrix key a runs-on reads, and each hosted Linux
// value the job's matrix gives that key, in source order. It answers no value
// when the matrix is not written out in the file, such as fromJSON of an output.
func matrixRunner(job *yaml.Node, value string) (string, []string) {
	m := matrixReference.FindStringSubmatch(value)
	if m == nil {
		return "", nil
	}
	key := m[1]
	matrix := mappingValue(mappingValue(job, "strategy"), "matrix")
	if matrix == nil || matrix.Kind != yaml.MappingNode {
		return "", nil
	}
	var values []*yaml.Node
	if list := mappingValue(matrix, key); list != nil {
		if list.Kind != yaml.SequenceNode {
			return "", nil
		}
		values = append(values, list.Content...)
	}
	if include := mappingValue(matrix, "include"); include != nil {
		if include.Kind != yaml.SequenceNode {
			return "", nil
		}
		for _, entry := range include.Content {
			if entry.Kind != yaml.MappingNode {
				return "", nil
			}
			if v := mappingValue(entry, key); v != nil {
				values = append(values, v)
			}
		}
	}
	var hosted []string
	for _, v := range values {
		if v.Kind == yaml.ScalarNode && hostedLinux.MatchString(v.Value) && !slices.Contains(hosted, v.Value) {
			hosted = append(hosted, v.Value)
		}
	}
	return key, hosted
}

// runsDocker reports whether a job needs a docker engine on its runner. It
// also names each action whose metadata could not be read. Such an action
// counts as a docker action, because only the docker runner runs both kinds.
func runsDocker(job *yaml.Node, dir string) (bool, []string) {
	if mappingValue(job, "services") != nil || mappingValue(job, "container") != nil {
		return true, nil
	}
	steps := mappingValue(job, "steps")
	if steps == nil || steps.Kind != yaml.SequenceNode {
		return false, nil
	}
	var unread []string
	for _, step := range steps.Content {
		if uses := mappingValue(step, "uses"); uses != nil {
			docker, why := dockerUses(strings.TrimSpace(uses.Value), dir)
			if why != "" {
				unread = append(unread, why)
			}
			if docker && why == "" {
				return true, nil
			}
		}
		run := mappingValue(step, "run")
		if run == nil {
			continue
		}
		for _, line := range strings.Split(run.Value, "\n") {
			if dockerCommand.MatchString(strings.TrimSpace(line)) {
				return true, nil
			}
		}
	}
	return len(unread) > 0, unread
}

// dockerUses reports whether the action that uses names runs in a container.
// The reason is not empty when the action metadata could not be read.
func dockerUses(uses, dir string) (bool, string) {
	lower := strings.ToLower(uses)
	if strings.HasPrefix(lower, "docker://") || dockerAction.MatchString(lower) {
		return true, ""
	}
	if strings.HasPrefix(uses, "./") || firstParty.MatchString(lower) {
		return false, ""
	}
	using, err := forkscope.Resolver{}.ActionUsing(dir, uses)
	if err != nil {
		return true, uses + ": " + err.Error()
	}
	return strings.EqualFold(using, "docker"), ""
}

// runnerExpression answers the runs-on value that sends a job to the org runners.
func runnerExpression(j hostedJob) string {
	name := runnerVar
	if j.docker {
		name = runnerDindVar
	}
	if j.key != "" {
		// A value outside the hosted Linux list keeps its runner. A public repository keeps every value.
		ref := "matrix." + j.key
		list, _ := json.Marshal(j.hosted)
		return "${{ (github.event.repository.visibility == 'public' || !contains(fromJSON('" + string(list) + "'), " + ref + ")) && " + ref + " || " + name + " || " + ref + " }}"
	}
	return "${{ github.event.repository.visibility == 'public' && '" + j.label + "' || " + name + " || '" + j.label + "' }}"
}

// privateRunners reports each job of an org repository whose runs-on ignores the visibility.
func privateRunners(path, content string) []ste.Finding {
	jobs, repo := governedJobs(path, content)
	var out []ste.Finding
	for _, j := range jobs {
		detail := repo + ": runs-on: " + j.node.Value
		if len(j.unread) > 0 {
			detail += " (the action metadata could not be read, so the rule chooses " + runnerDindVar + ", which has docker: " + strings.Join(j.unread, "; ") + ")"
		}
		out = append(out, ste.Finding{
			Line:   j.node.Line,
			ID:     IDPrivateRunner,
			Rule:   "an org job whose runner ignores the repository's visibility",
			Detail: detail,
			Fix:    "Use `runs-on: " + runnerExpression(j) + "`. GitHub bills each minute a private repository spends on its runners. `slopfix fix` does this.",
		})
	}
	return out
}

// retargetRunners writes the visibility expression into each job privateRunners reports.
func retargetRunners(path, content string) []edit.Edit {
	jobs, _ := governedJobs(path, content)
	fileLines := lines(content)
	var out []edit.Edit
	for _, j := range jobs {
		first, last, before, after, ok := scalarSource(content, fileLines, j)
		if !ok {
			continue
		}
		value := runnerExpression(j)
		if j.flow {
			value = `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
		}
		out = append(out, rewrite(content, first, last, []string{before + value + after}))
	}
	return out
}

// scalarSource answers the 0-based lines a job's runs-on value spans. It also
// answers the text on those lines before and after the value.
func scalarSource(content string, fileLines []string, j hostedJob) (first, last int, before, after string, ok bool) {
	first, col := j.node.Line-1, j.node.Column-1
	if first < 0 || first >= len(fileLines) || col < 0 || col > len(fileLines[first]) {
		return 0, 0, "", "", false
	}
	col += properties(fileLines[first][col:])
	before, head := fileLines[first][:col], fileLines[first][col:]
	style := j.node.Style &^ (yaml.FlowStyle | yaml.TaggedStyle)
	if style == yaml.LiteralStyle || style == yaml.FoldedStyle {
		last = flowEnd(content, fileLines, j.node.Line)
		return first, last, before, trailingComment.FindString(head), last >= first
	}
	if style == 0 && !j.flow {
		last, after, ok = plainSource(content, fileLines, j.node.Line, head)
		return first, last, before, after, ok
	}
	rest := strings.Join(append([]string{head}, fileLines[first+1:]...), "\n")
	end := scalarSpan(rest, style)
	if end < 0 {
		return 0, 0, "", "", false
	}
	taken := rest[:end]
	last = first + strings.Count(taken, "\n")
	if last == first {
		return first, last, before, head[end:], true
	}
	return first, last, before, fileLines[last][len(taken)-strings.LastIndexByte(taken, '\n')-1:], true
}

// properties answers the length of the tag and anchor that open head, with
// the blanks after them. The repair keeps them.
func properties(head string) int {
	at := 0
	for at < len(head) && (head[at] == '!' || head[at] == '&') {
		end := strings.IndexAny(head[at:], " \t")
		if end < 0 {
			return at
		}
		at += end
		for at < len(head) && (head[at] == ' ' || head[at] == '\t') {
			at++
		}
	}
	return at
}

// plainSource answers the last line of a plain scalar in block context, and
// the text after it. It runs to the line before the next node, and stops at
// a trailing comment.
func plainSource(content string, fileLines []string, line int, head string) (last int, after string, ok bool) {
	first := line - 1
	last = flowEnd(content, fileLines, line)
	if last < first {
		return 0, "", false
	}
	if last == first {
		end := scalarEnd(head, 0)
		return last, head[end:], true
	}
	if at := trailingComment.FindStringIndex(fileLines[last]); at != nil {
		after = fileLines[last][at[0]:]
	}
	return last, after, true
}

// scalarSpan answers the byte where the scalar that opens rest ends. A quoted
// scalar ends at its closing quote. A plain scalar inside a flow collection
// ends before a flow indicator or a comment.
func scalarSpan(rest string, style yaml.Style) int {
	switch style {
	case yaml.DoubleQuotedStyle, yaml.SingleQuotedStyle:
		return scalarEnd(rest, style)
	}
	end := len(rest)
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c == ',' || c == ']' || c == '}' || (c == '#' && i > 0 && strings.ContainsRune(" \t\n", rune(rest[i-1]))) {
			end = i
			break
		}
	}
	return len(strings.TrimRight(rest[:end], " \t\r\n"))
}
