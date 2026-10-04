package workflow

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
	yaml "go.yaml.in/yaml/v3"
)

// IDOrgActionRef is an org action or reusable workflow that names a ref other than master.
const IDOrgActionRef = "yaml/org-action-ref"

// orgOwner is the owner whose actions must track master.
const orgOwner = "wow-look-at-my"

// monorepo publishes each of its directories to its own orphan tag.
const monorepo = "actions"

// orphanTag is the tag the monorepo publishes a directory to from master.
var orphanTag = regexp.MustCompile(`^[A-Za-z0-9._-]+#latest$`)

// orgRef is a uses: value that names an org action.
type orgRef struct {
	node *yaml.Node
	// repo is the path before the @, with the owner.
	repo string
	ref  string
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
			out = append(out, orgRef{node: value, repo: repo, ref: ref})
		}
	})
	return out
}

// allowedRef reports whether a ref tracks master: master itself, or an orphan
// tag the monorepo publishes from it.
func allowedRef(ref string) bool {
	return ref == "master" || orphanTag.MatchString(ref)
}

// orgActionRefs reports each org action that names a ref other than master.
func orgActionRefs(content string) []ste.Finding {
	var out []ste.Finding
	for _, at := range orgRefs(content) {
		if allowedRef(at.ref) {
			continue
		}
		out = append(out, ste.Finding{
			Line:   at.node.Line,
			ID:     IDOrgActionRef,
			Rule:   "an org action that names a ref other than master",
			Detail: at.repo + "@" + at.ref,
			Fix:    "Use `@master`, or `@<directory>#latest` for an action of " + orgOwner + "/" + monorepo + ". `slopfix fix` does this.",
		})
	}
	return out
}

// masterRef answers the ref that tracks master for an action. A numbered
// orphan tag becomes its latest tag. A bare monorepo directory gets the tag
// the monorepo publishes it to.
func masterRef(repo, ref string) string {
	if name, _, ok := strings.Cut(ref, "#"); ok {
		return name + "#latest"
	}
	if strings.EqualFold(repo, orgOwner+"/"+monorepo) {
		return ref + "#latest"
	}
	return "master"
}

// retarget points each org action at the ref that tracks master. It rewrites
// the row the value sits on, and skips a value it cannot find there verbatim.
func retarget(content string) []edit.Edit {
	rows := lines(content)
	var out []edit.Edit
	for _, at := range orgRefs(content) {
		if allowedRef(at.ref) {
			continue
		}
		row, col := at.node.Line-1, at.node.Column-1
		if row < 0 || row >= len(rows) || col > len(rows[row]) {
			continue
		}
		old := at.repo + "@" + at.ref
		found := strings.Index(rows[row][col:], old)
		if found < 0 {
			continue
		}
		start := col + found
		swapped := rows[row][:start] + at.repo + "@" + masterRef(at.repo, at.ref) + rows[row][start+len(old):]
		out = append(out, rewrite(content, row, row, []string{swapped}))
	}
	return out
}
