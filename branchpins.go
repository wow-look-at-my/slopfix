// GitHubRefs answers workflow.Refs from the GitHub API, the way the fork scope reads it: GITHUB_API_URL.

package slopfix

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// GitHubRefs answers workflow.Refs from the GitHub API, the way the fork
// scope reads it: GITHUB_API_URL.
type GitHubRefs struct {
	getenv   func(string) string
	mu       sync.Mutex
	defaults map[string]string
	kinds    map[string]workflow.RefKind
}

// NewGitHubRefs reads the API settings through getenv.
func NewGitHubRefs(getenv func(string) string) *GitHubRefs {
	return &GitHubRefs{getenv: getenv, defaults: map[string]string{}, kinds: map[string]workflow.RefKind{}}
}

// DefaultBranch reads GET /repos/{repo}. A repository the token cannot read is
// the error, because its refs cannot be judged either.
func (g *GitHubRefs) DefaultBranch(repo string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if def, ok := g.defaults[repo]; ok {
		return def, nil
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := githubGetJSON(g.getenv, "/repos/"+repo, &info); err != nil {
		return "", fmt.Errorf("branch pin: %w", err)
	}
	if info.DefaultBranch == "" {
		return "", fmt.Errorf("branch pin: GET /repos/%s names no default branch", repo)
	}
	g.defaults[repo] = info.DefaultBranch
	return info.DefaultBranch, nil
}

// Kind asks GET /repos/{repo}/git/ref/heads/{ref}, then .../tags/{ref}. That
// endpoint matches a ref exactly.
func (g *GitHubRefs) Kind(repo, ref string) (workflow.RefKind, error) {
	if _, err := g.DefaultBranch(repo); err != nil {
		return workflow.RefMissing, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	key := repo + "@" + ref
	if kind, ok := g.kinds[key]; ok {
		return kind, nil
	}
	kind := workflow.RefMissing
	for _, try := range []struct {
		space string
		kind  workflow.RefKind
	}{{"heads", workflow.RefBranch}, {"tags", workflow.RefTag}} {
		path := "/repos/" + repo + "/git/ref/" + try.space + "/" + escapeRef(ref)
		status, body, err := githubGet(g.getenv, path)
		if err != nil {
			return workflow.RefMissing, fmt.Errorf("branch pin: %w", err)
		}
		if status == http.StatusOK {
			kind = try.kind
			break
		}
		if status != http.StatusNotFound {
			return workflow.RefMissing, fmt.Errorf("branch pin: GET %s: %d %s: %s", path, status, http.StatusText(status), strings.TrimSpace(string(body)))
		}
	}
	g.kinds[key] = kind
	return kind, nil
}

// escapeRef escapes each segment of a ref, so a # in an orphan-release tag
// reaches the API as part of the name.
func escapeRef(ref string) string {
	parts := strings.Split(ref, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// pinsInText writes the default branch over each branch pin of workflow text,
// when the request writes, and answers the text with the pins that remain. A
// lookup that cannot be made is a finding under the rule's own ID. No
// selection hides it and no failure reads as a clean file.
func pinsInText(req Request, text string) (string, []ste.Finding) {
	refs := req.Refs
	if refs == nil {
		refs = NewGitHubRefs(os.Getenv)
	}
	if !req.Scope.Bounded {
		fixed, err := workflow.RepairBranchPins(text, refs)
		if err != nil {
			return text, []ste.Finding{lookupFailure(err)}
		}
		text = fixed
	}
	found, err := workflow.BranchPins(text, refs)
	if err != nil {
		return text, []ste.Finding{lookupFailure(err)}
	}
	return text, found
}

// lookupFailure is the finding for a branch pin that could not be judged.
func lookupFailure(err error) ste.Finding {
	return ste.Finding{
		Line:   1,
		ID:     workflow.IDBranchPin,
		Rule:   "the branch pins could not be judged",
		Detail: err.Error(),
		Fix:    "Make the lookup answerable: reach the GitHub API and set GITHUB_TOKEN for a private repository.",
	}
}
