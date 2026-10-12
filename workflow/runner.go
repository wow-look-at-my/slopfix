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

// IDPrivateRunner is a job of a private org repository on a GitHub-hosted Linux runner.
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

// hostedLinux matches a GitHub-hosted x64 Linux label, which the org runners replace.
var hostedLinux = regexp.MustCompile(`^ubuntu-(latest|[0-9]+\.[0-9]+)$`)

// orgRunnerOnly matches a runs-on that sends a job to the org runners whatever the repository's visibility.
var orgRunnerOnly = regexp.MustCompile(`^\$\{\{\s*vars\.CI_RUNNER(?:_DIND)?\s*\|\|\s*'(ubuntu-(?:latest|[0-9]+\.[0-9]+))'\s*\}\}$`)

// dockerCommand matches a script line that runs the docker client.
var dockerCommand = regexp.MustCompile(`(^|[\s;&|(])(sudo\s+)?docker(-compose)?(\s|$)`)

// runnerRepoOf answers the org repository the file at path belongs to, and whether the rule applies.
// The visibility is read at run time by the expression the repair writes, so no lookup happens here.
func runnerRepoOf(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	dir := forkscope.ExistingDir(filepath.Dir(abs))
	name := forkscope.Repo{Name: forkscope.Resolver{}.RepoName(dir)}
	if !strings.EqualFold(name.Owner(), orgOwner) || SelfHostExempt.Contains(strings.ToLower(name.Name)) {
		return "", false
	}
	return name.Name, true
}

// hostedJob is a job whose runs-on names a GitHub-hosted Linux label, bare or as the fallback of an expression that ignores visibility.
type hostedJob struct {
	node   *yaml.Node
	label  string
	docker bool
}

// hostedJobs answers each job of content on a GitHub-hosted Linux label, in source order.
func hostedJobs(content string) []hostedJob {
	var doc yaml.Node
	if yaml.Unmarshal([]byte(content), &doc) != nil {
		return nil
	}
	jobs := mappingValue(rootOf(&doc), "jobs")
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
		label := on.Value
		if m := orgRunnerOnly.FindStringSubmatch(strings.TrimSpace(on.Value)); m != nil {
			label = m[1]
		} else if !hostedLinux.MatchString(on.Value) {
			continue
		}
		out = append(out, hostedJob{node: on, label: label, docker: runsDocker(job)})
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

// runnerExpression answers the runs-on value that keeps a public repository on
// the hosted label and sends every other run to the org runners. An event with
// no repository in its payload reads as not public.
func runnerExpression(j hostedJob) string {
	name := runnerVar
	if j.docker {
		name = runnerDindVar
	}
	return "${{ github.event.repository.visibility == 'public' && '" + j.label + "' || " + name + " || '" + j.label + "' }}"
}

// privateRunners reports each job of an org repository whose runner ignores the repository's visibility.
func privateRunners(path, content string) []ste.Finding {
	jobs := hostedJobs(content)
	if len(jobs) == 0 {
		return nil
	}
	repo, applies := runnerRepoOf(path)
	if !applies {
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

// retargetRunners writes the org runner expression into each job privateRunners reports.
func retargetRunners(path, content string) []edit.Edit {
	if len(privateRunners(path, content)) == 0 {
		return nil
	}
	fileLines := lines(content)
	var out []edit.Edit
	for _, j := range hostedJobs(content) {
		lineNo, col := j.node.Line-1, j.node.Column-1
		if lineNo < 0 || lineNo >= len(fileLines) || col >= len(fileLines[lineNo]) {
			continue
		}
		line := fileLines[lineNo]
		raw := scalarToken(line[col:])
		if raw == "" {
			continue
		}
		swapped := line[:col] + runnerExpression(j) + line[col+len(raw):]
		out = append(out, rewrite(content, lineNo, lineNo, []string{swapped}))
	}
	return out
}

// scalarToken answers the source text of the scalar at the start of rest.
// This covers a quoted scalar through its closing quote, or a plain one up
// to a space.
func scalarToken(rest string) string {
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, "${{") {
		end := strings.Index(rest, "}}")
		if end < 0 {
			return ""
		}
		return rest[:end+2]
	}
	if quote := rest[0]; quote == '\'' || quote == '"' {
		end := strings.IndexByte(rest[1:], quote)
		if end < 0 {
			return ""
		}
		return rest[:end+2]
	}
	if end := strings.IndexAny(rest, " \t"); end >= 0 {
		return rest[:end]
	}
	return rest
}
