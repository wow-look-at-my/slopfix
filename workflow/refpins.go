package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// IDBranchPin is a uses: ref that names a feature branch, or nothing a run can resolve.
const IDBranchPin = "yaml/branch-pin"

// RefKind is what a ref names in the repository that holds an action.
type RefKind int

const (
	// RefMissing names no branch and no tag.
	RefMissing RefKind = iota
	// RefBranch names a branch.
	RefBranch
	// RefTag names a tag.
	RefTag
)

// Refs answers what a ref names in an OWNER/NAME repository, and which branch
// is its default.
type Refs interface {
	Kind(repo, ref string) (RefKind, error)
	DefaultBranch(repo string) (string, error)
}

// Use is a uses: value that names an action or a workflow in another repository.
type Use struct {
	Line int
	// Repo is OWNER/NAME.
	Repo string
	// Ref follows the at sign.
	Ref string
	// Value is the whole uses: text.
	Value string
}

// fullSHA matches a full commit SHA, which needs no lookup.
var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Uses answers every remote uses: value of a workflow or an action manifest:
// a job's reusable workflow, a job's step, and a composite action's step. A
// local path, a docker:// image and a value that holds an expression name no
// remote ref. Unparseable YAML yields nothing, because the runner rejects it.
func Uses(content string) []Use {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	root := rootOf(&doc)
	var nodes []*yaml.Node
	stepUses := func(steps *yaml.Node) {
		if steps == nil || steps.Kind != yaml.SequenceNode {
			return
		}
		for _, step := range steps.Content {
			nodes = append(nodes, mappingValue(step, "uses"))
		}
	}
	if jobs := mappingValue(root, "jobs"); jobs != nil && jobs.Kind == yaml.MappingNode {
		for i := 1; i < len(jobs.Content); i += 2 {
			job := jobs.Content[i]
			nodes = append(nodes, mappingValue(job, "uses"))
			stepUses(mappingValue(job, "steps"))
		}
	}
	stepUses(mappingValue(mappingValue(root, "runs"), "steps"))
	var out []Use
	for _, node := range nodes {
		if use, ok := remoteUse(node); ok {
			out = append(out, use)
		}
	}
	return out
}

// remoteUse reads OWNER/NAME[/PATH]@REF from a uses: scalar.
func remoteUse(node *yaml.Node) (Use, bool) {
	if node == nil || node.Kind != yaml.ScalarNode {
		return Use{}, false
	}
	value := strings.TrimSpace(node.Value)
	if strings.HasPrefix(value, "./") || strings.HasPrefix(value, "docker://") || strings.Contains(value, "${{") {
		return Use{}, false
	}
	target, ref, ok := strings.Cut(value, "@")
	if !ok || ref == "" {
		return Use{}, false
	}
	parts := strings.Split(target, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Use{}, false
	}
	return Use{Line: node.Line, Repo: parts[0] + "/" + parts[1], Ref: ref, Value: value}, true
}

// BranchPins reports each uses: ref that names a branch other than the action
// repository's default branch. A merge deletes a feature branch, and every run
// that names it then fails to resolve the action. The default branch, a tag
// and a full commit SHA are refs that stay. A tag that the org's
// orphan-release cut from a branch build, ACTION/BRANCH#N, goes with that
// branch. It is a branch pin too. A ref that names no branch, no tag and no
// full SHA fails the run that resolves it. The first lookup that refs cannot
// answer is the error, because a guess would pass a pin it never checked.
func BranchPins(content string, refs Refs) ([]ste.Finding, error) {
	var out []ste.Finding
	for _, use := range Uses(content) {
		if fullSHA.MatchString(use.Ref) {
			continue
		}
		kind, err := refs.Kind(use.Repo, use.Ref)
		if err != nil {
			return nil, err
		}
		switch kind {
		case RefBranch:
			feature, err := featureBranch(use.Repo, use.Ref, refs)
			if err != nil {
				return nil, err
			}
			if feature {
				out = append(out, branchPinFinding(use, "names "+use.Ref+", a branch of "+use.Repo+" that is not its default branch"))
			}
		case RefMissing:
			out = append(out, branchPinFinding(use, use.Ref+" is neither a tag nor a branch of "+use.Repo+", and not a full commit SHA"))
		case RefTag:
			branch, err := tagBranch(use, refs)
			if err != nil {
				return nil, err
			}
			if branch != "" {
				out = append(out, branchPinFinding(use, "the tag "+use.Ref+" is a build of the branch "+branch+" of "+use.Repo+", and goes when that branch goes"))
			}
		}
	}
	return out, nil
}

// featureBranch reports whether branch is not the default branch of repo.
func featureBranch(repo, branch string, refs Refs) (bool, error) {
	def, err := refs.DefaultBranch(repo)
	if err != nil {
		return false, err
	}
	return branch != def, nil
}

// tagBranch answers the feature branch an orphan-release tag NAME#VERSION was
// cut from, or "" for a tag that follows none. The release tool names a branch
// build ACTION/BRANCH#VERSION. A branch name can hold a slash, so each tail of
// NAME after a slash is a candidate.
func tagBranch(use Use, refs Refs) (string, error) {
	hash := strings.LastIndex(use.Ref, "#")
	if hash < 0 {
		return "", nil
	}
	name := use.Ref[:hash]
	for i := strings.Index(name, "/"); i >= 0; {
		tail := name[i+1:]
		kind, err := refs.Kind(use.Repo, tail)
		if err != nil {
			return "", err
		}
		if kind == RefBranch {
			feature, err := featureBranch(use.Repo, tail, refs)
			if err != nil || feature {
				return tail, err
			}
			return "", nil
		}
		next := strings.Index(tail, "/")
		if next < 0 {
			break
		}
		i += next + 1
	}
	return "", nil
}

func branchPinFinding(use Use, detail string) ste.Finding {
	return ste.Finding{
		Line:   use.Line,
		ID:     IDBranchPin,
		Rule:   "a uses: ref names a feature branch, which a merge deletes",
		Detail: fmt.Sprintf("%s: %s", use.Value, detail),
		Fix:    "Pin " + use.Repo + " to its default branch, a tag it publishes, or a full commit SHA.",
	}
}
