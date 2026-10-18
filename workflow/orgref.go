package workflow

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/forkscope"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// IDOrgActionRef is an org action or reusable workflow that names a ref other than `lkgb`.
const IDOrgActionRef = "yaml/org-action-ref"

// lkgbTag is the last-known-good-build tag.
const lkgbTag = "lkgb"

// orgOwner is the owner whose actions must track their default branch.
const orgOwner = "wow-look-at-my"

// monorepo publishes each of its directories to its own orphan tag.
const monorepo = "actions"

// orphanTag is the tag the monorepo publishes a directory to from master.
var orphanTag = regexp.MustCompile(`^[A-Za-z0-9._-]+#latest$`)

// commitSHA is a full commit SHA, the only commit form a uses: value takes.
var commitSHA = regexp.MustCompile(`^[0-9A-Fa-f]{40}$`)

// commentName is the tag a pin's comment names: `# cache-upload#latest`.
var commentName = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9._-]*)#[A-Za-z0-9._-]+$`)

// versionName is a tag name, which no monorepo directory carries.
var versionName = regexp.MustCompile(`^[vV]?[0-9]`)

// orgRef is a uses: value that names an org action.
type orgRef struct {
	node *yaml.Node
	// repo is the path before the @, with the owner.
	repo string
	ref  string
	// comment is the line comment beside the value, without its marker.
	comment string
}

// orgRefs answers every uses: value that names an org action, in source order.
// Unparseable YAML yields nothing, because the runner rejects that file on its own.
func orgRefs(content string) []orgRef {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	var out []orgRef
	walk(&doc, func(node *yaml.Node) {
		if node.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			value := node.Content[i+1]
			if node.Content[i].Value != "uses" || value.Kind != yaml.ScalarNode {
				continue
			}
			repo, ref, ok := strings.Cut(value.Value, "@")
			owner, _, _ := strings.Cut(repo, "/")
			if !ok || !strings.EqualFold(owner, orgOwner) {
				continue
			}
			comment := value.LineComment
			if comment == "" {
				comment = node.Content[i].LineComment
			}
			comment = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment), "#"))
			out = append(out, orgRef{node: value, repo: repo, ref: ref, comment: comment})
		}
	})
	return out
}

// allowedRef reports the `lkgb` tag, or an orphan tag. The default branch passes only in a repository that publishes no `lkgb`.
// `<sha>#latest` names no tag.
func allowedRef(repo, ref string) bool {
	if ref == lkgbTag {
		return true
	}
	name, _, _ := strings.Cut(ref, "#")
	if orphanTag.MatchString(ref) && !commitSHA.MatchString(name) {
		return true
	}
	return ref == trackedBranch(repo) && stableRef(repo) != lkgbTag
}

// stableRef answers `lkgb` when the repository publishes it, and its default branch otherwise. `lkgb` is the last master commit that passed CI, so a red master does not reach the caller.
// Where GitHub cannot answer, it answers the default branch.
func stableRef(repo string) string {
	parts := strings.SplitN(repo, "/", 3)
	if len(parts) >= 2 {
		if has, err := (forkscope.Resolver{}).HasTag(parts[0]+"/"+parts[1], lkgbTag); err == nil && has {
			return lkgbTag
		}
	}
	return trackedBranch(repo)
}

// trackedBranch answers the default branch of the repository that holds an
// action path. Where GitHub cannot answer, it answers master, the org's branch.
func trackedBranch(repo string) string {
	parts := strings.SplitN(repo, "/", 3)
	if len(parts) < 2 {
		return "master"
	}
	branch, err := forkscope.Resolver{}.DefaultBranch(parts[0] + "/" + parts[1])
	if err != nil {
		return "master"
	}
	return branch
}

// isMonorepo reports whether a repo path is the monorepo itself, not a path inside it.
func isMonorepo(repo string) bool {
	return strings.EqualFold(repo, orgOwner+"/"+monorepo)
}

// pinnedDirectory answers the monorepo directory a SHA pin stands for. Only the
// comment beside the pin can name it. The second result is false when no
// comment names one, because a guessed directory breaks the job.
func pinnedDirectory(at orgRef) (string, bool) {
	fields := strings.Fields(at.comment)
	if len(fields) == 0 {
		return "", false
	}
	match := commentName.FindStringSubmatch(fields[0])
	if match == nil || versionName.MatchString(match[1]) {
		return "", false
	}
	return match[1], true
}

// repairable reports whether the rule can name the ref that tracks master. A
// SHA pin of the monorepo with no directory beside it has no such ref.
func repairable(at orgRef) bool {
	name, _, _ := strings.Cut(at.ref, "#")
	if !isMonorepo(at.repo) || !commitSHA.MatchString(name) {
		return true
	}
	_, ok := pinnedDirectory(at)
	return ok
}

// orgActionRefs reports each org action that names a ref other than `lkgb` or
// an orphan tag.
func orgActionRefs(content string) []ste.Finding {
	var out []ste.Finding
	for _, at := range orgRefs(content) {
		if allowedRef(at.repo, at.ref) || !repairable(at) {
			continue
		}
		out = append(out, ste.Finding{
			Line:   at.node.Line,
			ID:     IDOrgActionRef,
			Rule:   "an org action that names a ref other than `" + lkgbTag + "`",
			Detail: at.repo + "@" + at.ref,
			Fix:    "Use `@" + stableRef(at.repo) + "`, or `@<directory>#latest` for an action of " + orgOwner + "/" + monorepo + ". `slopfix fix` writes `" + lkgbTag + "`, or the default branch of a repository that publishes no `" + lkgbTag + "`, or the directory tag.",
		})
	}
	return out
}

// masterRef answers the ref that tracks master for an action: `lkgb`, or the
// default branch of a repository that publishes no `lkgb`. A numbered orphan
// tag becomes its latest tag. A bare monorepo directory gets the tag the
// monorepo publishes it to. A monorepo SHA pin gets the tag of the directory
// its comment names.
func masterRef(at orgRef) string {
	name, _, tagged := strings.Cut(at.ref, "#")
	if commitSHA.MatchString(name) {
		if dir, ok := pinnedDirectory(at); ok && isMonorepo(at.repo) {
			return dir + "#latest"
		}
		return stableRef(at.repo)
	}
	if tagged {
		return name + "#latest"
	}
	if isMonorepo(at.repo) && at.ref != trackedBranch(at.repo) {
		return at.ref + "#latest"
	}
	return stableRef(at.repo)
}

// retarget points each org action at the ref that tracks master. It rewrites
// the line the value sits on, and skips a value it cannot find there verbatim.
func retarget(content string) []edit.Edit {
	fileLines := lines(content)
	var out []edit.Edit
	for _, at := range orgRefs(content) {
		if allowedRef(at.repo, at.ref) || !repairable(at) {
			continue
		}
		lineNo, col := at.node.Line-1, at.node.Column-1
		if lineNo < 0 || lineNo >= len(fileLines) || col > len(fileLines[lineNo]) {
			continue
		}
		old := at.repo + "@" + at.ref
		found := strings.Index(fileLines[lineNo][col:], old)
		if found < 0 {
			continue
		}
		start := col + found
		swapped := fileLines[lineNo][:start] + at.repo + "@" + masterRef(at) + fileLines[lineNo][start+len(old):]
		out = append(out, rewrite(content, lineNo, lineNo, []string{swapped}))
	}
	return out
}
