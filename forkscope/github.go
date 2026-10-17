package forkscope

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// forkParent is a fork's parent as GET /repos/{repo} names it. GitHub gives
// clone_url; a mirror that drops URL fields gives only full_name.
type forkParent struct {
	CloneURL      string `json:"clone_url"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

// forkRepo is the part of GET /repos/{repo} that names a parent.
type forkRepo struct {
	Fork   *bool       `json:"fork"`
	Parent *forkParent `json:"parent"`
}

// parentURL answers the URL a fork's parent clones from.
func (r Resolver) parentURL(p *forkParent) string {
	if p.CloneURL != "" {
		return p.CloneURL
	}
	if p.FullName == "" {
		return ""
	}
	server := strings.TrimRight(r.getenv("GITHUB_SERVER_URL"), "/")
	if server == "" {
		server = DefaultServer
	}
	return server + "/" + p.FullName + ".git"
}

// tokenVars are the variables a GitHub token comes from, in the order they win: the one Actions sets.
var tokenVars = []string{"GITHUB_TOKEN", "GH_TOKEN"}

// token answers the bearer for the API, or empty when no variable carries one.
func (r Resolver) token() string {
	for _, name := range tokenVars {
		if token := r.getenv(name); token != "" {
			return token
		}
	}
	return ""
}

// fetchRepo reads GET /repos/{repo}. With no token and the default API, the gh
// CLI asks with the user's own credentials, which see a private repository and
// are not held to the anonymous rate limit. Otherwise the API is asked
// directly, with the token as the bearer when one is set.
func (r Resolver) fetchRepo(repo string) (forkRepo, error) {
	if r.token() == "" && r.getenv("GITHUB_API_URL") == "" {
		return r.ghRepo(repo)
	}
	url := r.api() + "/repos/" + repo
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return forkRepo{}, fmt.Errorf("fork scope: GET %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	token := r.token()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return forkRepo{}, fmt.Errorf("fork scope: GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return forkRepo{}, fmt.Errorf("fork scope: GET %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK && token == "" {
		return forkRepo{}, fmt.Errorf("fork scope: GET %s: %s: %s (sent anonymously: set %s to authenticate)", url, resp.Status, strings.TrimSpace(string(body)), strings.Join(tokenVars, " or "))
	}
	if resp.StatusCode != http.StatusOK {
		return forkRepo{}, fmt.Errorf("fork scope: GET %s: %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}
	return decodeRepo(body, "GET "+url)
}

// ghRepoQuery asks GraphQL whether a repository is a fork, and of what.
const ghRepoQuery = `query($owner: String!, $name: String!) { repository(owner: $owner, name: $name) { isFork parent { nameWithOwner defaultBranchRef { name } } } }`

// ghRepoShape reshapes the GraphQL answer into the REST repository object's fork fields.
const ghRepoShape = `.data.repository | {fork: .isFork, parent: (if .parent then {full_name: .parent.nameWithOwner, default_branch: .parent.defaultBranchRef.name} else null end)}`

// ghRepo asks GraphQL through the gh CLI. GraphQL states isFork on every
// server gh may point at, a mirror's trimmed REST rebuild included.
func (r Resolver) ghRepo(repo string) (forkRepo, error) {
	run := r.GH
	if run == nil {
		run = runGH
	}
	owner, name, _ := strings.Cut(repo, "/")
	out, err := run("api", "graphql", "-f", "query="+ghRepoQuery, "-F", "owner="+owner, "-F", "name="+name, "--jq", ghRepoShape)
	if err != nil {
		return forkRepo{}, fmt.Errorf("fork scope: gh api graphql for %s: %w", repo, err)
	}
	return decodeRepo(out, "gh api graphql for "+repo)
}

// decodeRepo reads a repository object. An answer with no fork flag is an
// error: read as false, it would call every fork no fork.
func decodeRepo(body []byte, from string) (forkRepo, error) {
	var info forkRepo
	if err := json.Unmarshal(body, &info); err != nil {
		return forkRepo{}, fmt.Errorf("fork scope: %s: %w", from, err)
	}
	if info.Fork == nil {
		return forkRepo{}, fmt.Errorf("fork scope: %s answered a repository with no fork flag", from)
	}
	return info, nil
}

// runGH runs the gh CLI and answers its output, or an error that quotes its stderr.
func runGH(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", args...).Output()
	if err == nil {
		return out, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exit.Stderr)))
	}
	return nil, err
}
