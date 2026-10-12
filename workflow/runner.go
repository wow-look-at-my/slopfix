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

// dockerCommand matches a script line that runs the docker client.
var dockerCommand = regexp.MustCompile(`(^|[\s;&|(])(sudo\s+)?docker(-compose)?(\s|$)`)

// dockerAction matches a uses: value whose action runs the docker client. The org's ghcr action builds with buildx.
var dockerAction = regexp.MustCompile(`docker|buildx|^wow-look-at-my/actions@ghcr#`)

// firstParty matches an action owner whose actions the rule knows. Their docker use is in dockerAction.
var firstParty = regexp.MustCompile(`^(actions|wow-look-at-my)/`)

// runnerTarget is the repository a file belongs to, as the rule needs it.
type runnerTarget struct {
	repo forkscope.Repo
	// dir is a directory of the work tree. The action lookups keep their answers there.
	dir string
	// unread is why the visibility could not be read. The rule then treats the repository as private.
	unread string
}

// runnerTargetOf answers the target of the file at path, and whether the rule applies.
func runnerTargetOf(path string) (runnerTarget, bool) {
	if path == "" {
		return runnerTarget{}, false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return runnerTarget{}, false
	}
	dir := forkscope.ExistingDir(filepath.Dir(abs))
	// An exempt repository is skipped before the network is asked.
	name := forkscope.Repo{Name: forkscope.Resolver{}.RepoName(dir)}
	if !strings.EqualFold(name.Owner(), orgOwner) || SelfHostExempt.Contains(strings.ToLower(name.Name)) {
		return runnerTarget{}, false
	}
	repo, err := forkscope.Resolver{}.Visibility(dir, orgOwner)
	if err != nil {
		return runnerTarget{repo: repo, dir: dir, unread: err.Error()}, true
	}
	return runnerTarget{repo: repo, dir: dir}, repo.Private
}

// hostedJob is a job whose runs-on names a GitHub-hosted Linux label.
type hostedJob struct {
	job    *yaml.Node
	node   *yaml.Node
	label  string
	docker bool
	// unread names each action whose runs.using could not be read. The job then goes to the docker runner.
	unread []string
}

// governedJobs answers each hosted job of the file at path that the rule
// governs, with its docker need, and the file's target.
func governedJobs(path, content string) ([]hostedJob, runnerTarget) {
	jobs := hostedJobs(content)
	if len(jobs) == 0 {
		return nil, runnerTarget{}
	}
	target, applies := runnerTargetOf(path)
	if !applies {
		return nil, runnerTarget{}
	}
	for idx := range jobs {
		jobs[idx].docker, jobs[idx].unread = runsDocker(jobs[idx].job, target.dir)
	}
	return jobs, target
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
		if on == nil || on.Kind != yaml.ScalarNode || !hostedLinux.MatchString(on.Value) {
			continue
		}
		out = append(out, hostedJob{job: job, node: on, label: on.Value})
	}
	return out
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
	return "${{ " + name + " || '" + j.label + "' }}"
}

// privateRunners reports each job of a private org repository on a GitHub-hosted Linux runner.
func privateRunners(path, content string) []ste.Finding {
	jobs, target := governedJobs(path, content)
	var out []ste.Finding
	for _, j := range jobs {
		detail := target.repo.Name + ": runs-on: " + j.label
		if target.unread != "" {
			detail += " (the visibility could not be read, so the rule treats the repository as private: " + target.unread + ")"
		}
		if len(j.unread) > 0 {
			detail += " (the action metadata could not be read, so the rule chooses " + runnerDindVar + ", which has docker: " + strings.Join(j.unread, "; ") + ")"
		}
		out = append(out, ste.Finding{
			Line:   j.node.Line,
			ID:     IDPrivateRunner,
			Rule:   "a private org repository that runs a job on a GitHub-hosted runner",
			Detail: detail,
			Fix:    "Use `runs-on: " + runnerExpression(j) + "`. GitHub bills each minute a private repository spends on its runners. `slopfix fix` does this.",
		})
	}
	return out
}

// retargetRunners writes the org runner expression into each job privateRunners reports.
func retargetRunners(path, content string) []edit.Edit {
	jobs, _ := governedJobs(path, content)
	fileLines := lines(content)
	var out []edit.Edit
	for _, j := range jobs {
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
