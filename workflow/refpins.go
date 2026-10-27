// IDBranchPin is a uses: ref that names a feature branch, or nothing a run can resolve.

package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
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

// Uses answers every remote uses: value the runner reads. A local path, a docker:// image and an expression name no remote ref.
func Uses(content string) []Use {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	var out []Use
	for _, site := range usesSites(&doc) {
		if use, ok := remoteUse(site.value); ok {
			out = append(out, use)
		}
	}
	return out
}

// usesSite is one uses: key that the runner reads, and its value.
type usesSite struct {
	key, value *yaml.Node
}

// usesSites answers each uses: key the runner reads, in source order: a job's reusable workflow, a job's step, and a composite action's step.
// A uses: key anywhere else, such as an input under with:, names no action.
func usesSites(doc *yaml.Node) []usesSite {
	root := rootOf(doc)
	var out []usesSite
	add := func(node *yaml.Node) {
		if node == nil || node.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "uses" {
				out = append(out, usesSite{key: node.Content[i], value: node.Content[i+1]})
			}
		}
	}
	stepUses := func(steps *yaml.Node) {
		if steps == nil || steps.Kind != yaml.SequenceNode {
			return
		}
		for _, step := range steps.Content {
			add(step)
		}
	}
	if jobs := mappingValue(root, "jobs"); jobs != nil && jobs.Kind == yaml.MappingNode {
		for i := 1; i < len(jobs.Content); i += 2 {
			job := jobs.Content[i]
			add(job)
			stepUses(mappingValue(job, "steps"))
		}
	}
	stepUses(mappingValue(mappingValue(root, "runs"), "steps"))
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
// repository's default branch. A merge deletes a feature branch, and every
// run that names it then fails to resolve the action.
func BranchPins(content string, refs Refs) ([]ste.Finding, error) {
	pins, err := branchPins(content, refs)
	if err != nil {
		return nil, err
	}
	var out []ste.Finding
	for _, p := range pins {
		out = append(out, branchPinFinding(p.use, p.detail))
	}
	return out, nil
}

// branchPin is a uses: value that BranchPins reports, and the ref the repair
// writes in its place.
type branchPin struct {
	use    Use
	detail string
	// ref is the default branch, or for a branch build's tag the tag that follows the default branch.
	ref string
}

// branchPins answers every reported uses: value with its replacement ref.
func branchPins(content string, refs Refs) ([]branchPin, error) {
	var out []branchPin
	for _, use := range Uses(content) {
		if fullSHA.MatchString(use.Ref) || ownedByOrgActionRef(use) {
			continue
		}
		kind, err := refs.Kind(use.Repo, use.Ref)
		if err != nil {
			return nil, err
		}
		def, err := refs.DefaultBranch(use.Repo)
		if err != nil {
			return nil, err
		}
		switch kind {
		case RefBranch:
			if use.Ref != def {
				out = append(out, branchPin{use, "names " + use.Ref + ", a branch of " + use.Repo + " that is not its default branch", def})
			}
		case RefMissing:
			out = append(out, branchPin{use, use.Ref + " is neither a tag nor a branch of " + use.Repo + ", and not a full commit SHA", def})
		case RefTag:
			branch, err := tagBranch(use, refs)
			if err != nil {
				return nil, err
			}
			if branch != "" {
				name := strings.TrimSuffix(use.Ref[:strings.LastIndex(use.Ref, "#")], "/"+branch)
				out = append(out, branchPin{use, "the tag " + use.Ref + " is a build of the branch " + branch + " of " + use.Repo + ", and goes when that branch goes", name + "#latest"})
			}
		}
	}
	return out, nil
}

// ownedByOrgActionRef reports an org action at a ref that names no orphan tag.
func ownedByOrgActionRef(use Use) bool {
	owner, _, _ := strings.Cut(use.Repo, "/")
	return strings.EqualFold(owner, orgOwner) && !strings.Contains(use.Ref, "#")
}

// RetargetBranchPins writes each reported uses: value again with its
// replacement ref: the default branch of the action's repository, or for a
// branch build's tag. The tag that follows the default branch. It edits the
// row the value sits on and skips a value it cannot find there verbatim.
func RetargetBranchPins(content string, refs Refs) ([]edit.Edit, error) {
	pins, err := branchPins(content, refs)
	if err != nil {
		return nil, err
	}
	fileLines := lines(content)
	var out []edit.Edit
	for _, p := range pins {
		row := p.use.Line - 1
		if row < 0 || row >= len(fileLines) || !strings.Contains(fileLines[row], p.use.Value) {
			continue
		}
		target, _, _ := strings.Cut(p.use.Value, "@")
		swapped := strings.Replace(fileLines[row], p.use.Value, target+"@"+p.ref, 1)
		out = append(out, rewrite(content, row, row, []string{swapped}))
	}
	return out, nil
}

// RepairBranchPins answers the workflow with every branch pin retargeted,
// written through the gates slopfix fix uses.
func RepairBranchPins(content string, refs Refs) (string, error) {
	edits, err := RetargetBranchPins(content, refs)
	if err != nil || len(edits) == 0 {
		return content, err
	}
	f := fixer.NewFile("", content, Options(fixer.Options{}))
	f.Apply(edits)
	return f.Text(), nil
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
		Fix:    "Pin " + use.Repo + " to its default branch, a tag it publishes, or a full commit SHA. `slopfix fix` writes the default branch, or the latest tag of a branch build.",
	}
}
