package workflow

import (
	"path/filepath"
	"regexp"
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
	"wow-look-at-my/buildhost",
	"wow-look-at-my/github-state-mirror",
)

// hostedLinux matches a GitHub-hosted x64 Linux label.
var hostedLinux = regexp.MustCompile(`^ubuntu-(latest|[0-9]+\.[0-9]+)$`)

// blindFallback matches an expression that picks an org runner variable.
var blindFallback = regexp.MustCompile(`^\$\{\{\s*vars\.(CI_RUNNER|CI_RUNNER_DIND)\s*\|\|\s*'(ubuntu-(?:latest|[0-9]+\.[0-9]+))'\s*\}\}$`)

// dockerCommand matches a script line that runs the docker client.
var dockerCommand = regexp.MustCompile(`(^|[\s;&|(])(sudo\s+)?docker(-compose)?(\s|$)`)

// governedRepo answers the org repository the file at path belongs to, and
// whether the rule reads it. The name comes from the work tree alone.
func governedRepo(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	repo := forkscope.Resolver{}.RepoName(forkscope.ExistingDir(filepath.Dir(abs)))
	owner, _, _ := strings.Cut(repo, "/")
	if !strings.EqualFold(owner, orgOwner) || SelfHostExempt.Contains(strings.ToLower(repo)) {
		return "", false
	}
	return repo, true
}

// hostedJob is a job whose runs-on ignores the repository's visibility.
type hostedJob struct {
	node   *yaml.Node
	label  string
	docker bool
	// flow reports whether the runs-on value sits inside a flow collection.
	flow bool
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

// hostedJobs answers each job of content whose runs-on ignores the visibility, in source order.
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
		label, dind := runnerLabel(on.Value)
		if label == "" {
			continue
		}
		flow := (root.Style|jobs.Style|job.Style)&yaml.FlowStyle != 0
		out = append(out, hostedJob{node: on, label: label, docker: dind || runsDocker(job), flow: flow})
	}
	return out
}

// runsDocker reports whether a job needs a docker engine on its runner.
func runsDocker(job *yaml.Node) bool {
	if mappingValue(job, "services") != nil || mappingValue(job, "container") != nil {
		return true
	}
	steps := mappingValue(job, "steps")
	if steps == nil || steps.Kind != yaml.SequenceNode {
		return false
	}
	for _, step := range steps.Content {
		if uses := mappingValue(step, "uses"); uses != nil && strings.Contains(strings.ToLower(uses.Value), "docker") {
			return true
		}
		run := mappingValue(step, "run")
		if run == nil {
			continue
		}
		for _, line := range strings.Split(run.Value, "\n") {
			if dockerCommand.MatchString(strings.TrimSpace(line)) {
				return true
			}
		}
	}
	return false
}

// runnerExpression answers the runs-on value that sends a job to the org runners.
func runnerExpression(j hostedJob) string {
	name := runnerVar
	if j.docker {
		name = runnerDindVar
	}
	return "${{ github.event.repository.visibility == 'public' && '" + j.label + "' || " + name + " || '" + j.label + "' }}"
}

// privateRunners reports each job of an org repository whose runs-on ignores the visibility.
func privateRunners(path, content string) []ste.Finding {
	jobs := hostedJobs(content)
	if len(jobs) == 0 {
		return nil
	}
	repo, governed := governedRepo(path)
	if !governed {
		return nil
	}
	var out []ste.Finding
	for _, j := range jobs {
		out = append(out, ste.Finding{
			Line:   j.node.Line,
			ID:     IDPrivateRunner,
			Rule:   "an org job whose runner ignores the repository's visibility",
			Detail: repo + ": runs-on: " + j.node.Value,
			Fix:    "Use `runs-on: " + runnerExpression(j) + "`. GitHub bills each minute a private repository spends on its runners. `slopfix fix` does this.",
		})
	}
	return out
}

// retargetRunners writes the visibility expression into each job privateRunners reports.
func retargetRunners(path, content string) []edit.Edit {
	if len(privateRunners(path, content)) == 0 {
		return nil
	}
	fileLines := lines(content)
	var out []edit.Edit
	for _, j := range hostedJobs(content) {
		first, last, before, after, ok := scalarSource(content, fileLines, j)
		if !ok {
			continue
		}
		value := runnerExpression(j)
		if j.flow {
			value = `"` + value + `"`
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
