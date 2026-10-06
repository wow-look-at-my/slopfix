// Package forkscope tells which lines of a fork's work tree the fork wrote.
//
// A fork carries its parent's whole tree, and the parent never agreed to these
// rules. The repository is the one the work tree's origin names on the GitHub
// server. With no such origin, it is GITHUB_REPOSITORY, for a root inside
// GITHUB_WORKSPACE. A repository that the org's fork list names is measured
// from the newest upstream tag that HEAD contains. With no such tag, it is
// measured from the upstream commit whose tree is closest to HEAD's. This is
// the snapshot a squashed sync brought in. Otherwise the API says whether the
// repository is a fork, and the base is the merge base with the parent's
// default branch. Only a line added or changed since that base, or a line of
// a new or untracked file, is the fork's.
package forkscope

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/jsonc"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitread"
)

// DefaultGitHubAPI is the API root when GITHUB_API_URL is unset.
const DefaultGitHubAPI = "https://api.github.com"

// DefaultServer is the GitHub server when GITHUB_SERVER_URL is unset.
const DefaultServer = "https://github.com"

// ListURL serves the fork list that the org's dot_github repository publishes to buildhost.
const ListURL = "https://sites.pazer.build/dot_github/fork-of.json"

// CacheTTL is how long a work tree keeps what the network said about its fork.
const CacheTTL = time.Hour

// cacheFile is the name, in the git common directory, of that record.
const cacheFile = "slopfix-fork.json"

// Resolver finds a fork's base. The zero Resolver reads the process
// environment and the org's fork list.
type Resolver struct {
	Getenv  func(string) string
	ListURL string
	// GH runs the gh CLI; nil runs the one on PATH.
	GH func(args ...string) ([]byte, error)
}

func (r Resolver) getenv(key string) string {
	if r.Getenv == nil {
		return os.Getenv(key)
	}
	return r.Getenv(key)
}

func (r Resolver) listURL() string {
	if r.ListURL == "" {
		return ListURL
	}
	return r.ListURL
}

// Base is the commit a fork's own lines count from, in the work tree at top.
type Base struct {
	top    string
	repo   *gitread.Repo
	commit string
	// upstream is the tip of a listed fork's upstream. A file that matches a version of it is upstream's.
	upstream string
}

// Commit answers the base commit.
func (b *Base) Commit() string { return b.commit }

// Top answers the work tree root.
func (b *Base) Top() string { return b.top }

// Kinds of repository a record names.
const (
	kindPlain  = "plain"
	kindListed = "listed"
	kindParent = "parent"
)

// record is what the network says about a repository's fork. It is cached in
// the git common directory, so a hook that runs on every write asks once.
type record struct {
	Repo    string    `json:"repo"`
	Source  string    `json:"source"`
	Checked time.Time `json:"checked"`
	Kind    string    `json:"kind"`
	// Upstream and Tags are a listed fork's upstream URL and the commit each tag names.
	Upstream string   `json:"upstream,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	// Tip is the commit of the upstream's default branch, fetched when no tag is in HEAD's history.
	Tip string `json:"tip,omitempty"`
	// ParentURL, Branch and Parent are a GitHub fork's parent, its default branch and that branch's commit.
	ParentURL string `json:"parent_url,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Parent    string `json:"parent,omitempty"`
}

// identity is the part of a record that needs no work tree.
type identity struct {
	kind      string
	upstream  string
	parentURL string
	branch    string
}

// identities remembers each repository's identity for the life of the process.
var identities = struct {
	sync.Mutex
	byKey map[string]identity
}{byKey: map[string]identity{}}

// Lines answers the lines a fork wrote, for the work tree that holds root, or
// nil when root is in no fork.
func (r Resolver) Lines(root string) (*Lines, error) {
	base, err := r.Base(root)
	if err != nil || base == nil {
		return nil, err
	}
	return base.Lines()
}

// Base answers the fork base of the work tree that holds root, or nil when
// root is in no fork. A fork with no base is an error.
func (r Resolver) Base(root string) (*Base, error) {
	repo, err := openRepo(root)
	top := ""
	if repo != nil {
		top = repo.WorkTree()
	}
	name := ""
	if repo != nil {
		name = r.originRepo(repo)
	}
	if name == "" && r.workspaceHolds(root) {
		name = r.getenv("GITHUB_REPOSITORY")
	}
	if name == "" {
		return nil, nil
	}
	rec, err := r.record(repo, top, err, name)
	if err != nil {
		return nil, err
	}
	switch rec.Kind {
	case kindListed:
		commit, err := tagBase(repo, rec.Tags)
		if err != nil {
			return nil, err
		}
		if commit == "" {
			if commit, err = snapshotBase(repo, &rec); err != nil {
				return nil, err
			}
		}
		return &Base{top: top, repo: repo, commit: commit, upstream: rec.Tip}, nil
	case kindParent:
		if err := deepen(repo); err != nil {
			return nil, err
		}
		head, err := repo.Head()
		if err != nil {
			return nil, fmt.Errorf("fork scope: %w", err)
		}
		parent, err := gitread.ParseOID(rec.Parent)
		if err != nil {
			return nil, fmt.Errorf("fork scope: %w", err)
		}
		base, ok, err := repo.MergeBase(head, parent)
		if err != nil || !ok {
			return nil, fmt.Errorf("fork scope: no merge base between HEAD and the parent's %s (%s) from %s", rec.Branch, rec.Parent, rec.ParentURL)
		}
		return &Base{top: top, repo: repo, commit: base.String()}, nil
	}
	return nil, nil
}

// openRepo opens the work tree that holds dir, or answers an error that names
// the git command.
func openRepo(dir string) (*gitread.Repo, error) {
	repo, err := gitread.OpenWorkTree(dir)
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	if repo == nil {
		return nil, fmt.Errorf("fork scope: git rev-parse --show-toplevel: %s is not inside a work tree", dir)
	}
	return repo, nil
}

// workspaceHolds reports whether GITHUB_REPOSITORY names the checkout that
// holds root: GITHUB_WORKSPACE is unset, or root lies inside it.
func (r Resolver) workspaceHolds(root string) bool {
	workspace := r.getenv("GITHUB_WORKSPACE")
	if workspace == "" {
		return true
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(realPath(workspace), realPath(abs))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// realPath answers path with every link resolved, or path when it does not resolve.
func realPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// originRepo answers the OWNER/NAME the origin remote of top names on the
// GitHub server, or "" for an origin elsewhere or none.
func (r Resolver) originRepo(repo *gitread.Repo) string {
	raw := repo.OriginURL()
	if raw == "" {
		return ""
	}
	server := r.getenv("GITHUB_SERVER_URL")
	if server == "" {
		server = DefaultServer
	}
	serverURL, err := url.Parse(server)
	if err != nil {
		return ""
	}
	return repoOfURL(strings.TrimSpace(raw), serverURL.Hostname())
}

// repoOfURL answers the OWNER/NAME a clone URL names on host, or "".
func repoOfURL(raw, host string) string {
	var gotHost, path string
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return ""
		}
		gotHost, path = u.Hostname(), u.Path
	} else {
		at, rest, ok := strings.Cut(raw, ":")
		if !ok {
			return ""
		}
		if _, h, hasUser := strings.Cut(at, "@"); hasUser {
			at = h
		}
		gotHost, path = at, rest
	}
	if !strings.EqualFold(gotHost, host) {
		return ""
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	owner, name, ok := strings.Cut(path, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return ""
	}
	return owner + "/" + name
}

// source names where a record's answers came from, so a record another API or
// list wrote is not read as this's.
func (r Resolver) source() string {
	return r.api() + " " + r.listURL()
}

func (r Resolver) api() string {
	api := strings.TrimRight(r.getenv("GITHUB_API_URL"), "/")
	if api == "" {
		return DefaultGitHubAPI
	}
	return api
}

// record answers what the network says about repo, from the cache when it is fresh.
func (r Resolver) record(g *gitread.Repo, top string, topErr error, repo string) (record, error) {
	if topErr == nil {
		if rec, ok := r.cached(g, repo); ok {
			return rec, nil
		}
	}
	id, err := r.identify(repo)
	if err != nil {
		return record{}, err
	}
	rec := record{Repo: repo, Source: r.source(), Checked: time.Now(), Kind: id.kind}
	switch id.kind {
	case kindPlain:
		if topErr != nil {
			return rec, nil
		}
	case kindListed:
		if topErr != nil {
			return record{}, topErr
		}
		rec.Upstream = id.upstream
		if rec.Tags, err = upstreamTags(id.upstream); err != nil {
			return record{}, err
		}
	case kindParent:
		if topErr != nil {
			return record{}, topErr
		}
		rec.ParentURL, rec.Branch = id.parentURL, id.branch
		if rec.Parent, err = fetchParent(g, id.parentURL, id.branch); err != nil {
			return record{}, err
		}
	}
	if err := store(g, rec); err != nil {
		return record{}, err
	}
	return rec, nil
}

// identify answers whether repo is a listed fork, a GitHub fork, or neither.
func (r Resolver) identify(repo string) (identity, error) {
	key := r.source() + " " + strings.ToLower(repo)
	identities.Lock()
	id, ok := identities.byKey[key]
	identities.Unlock()
	if ok {
		return id, nil
	}
	upstream, err := listedUpstream(repo, r.listURL())
	if err != nil {
		return identity{}, err
	}
	switch {
	case upstream != "":
		id = identity{kind: kindListed, upstream: upstream}
	case r.eventSaysPlain(repo):
		id = identity{kind: kindPlain}
	default:
		info, err := r.fetchRepo(repo)
		if err != nil {
			return identity{}, err
		}
		id = identity{kind: kindPlain}
		if *info.Fork {
			if info.Parent == nil || r.parentURL(info.Parent) == "" || info.Parent.DefaultBranch == "" {
				return identity{}, fmt.Errorf("fork scope: %s is a fork, but the API names no parent repository and default branch", repo)
			}
			id = identity{kind: kindParent, parentURL: r.parentURL(info.Parent), branch: info.Parent.DefaultBranch}
		}
	}
	identities.Lock()
	identities.byKey[key] = id
	identities.Unlock()
	return id, nil
}

// eventSaysPlain reports whether the Actions event payload at GITHUB_EVENT_PATH
// describes repo as no fork, which spares the API call.
func (r Resolver) eventSaysPlain(repo string) bool {
	path := r.getenv("GITHUB_EVENT_PATH")
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var event struct {
		Repository *struct {
			FullName string `json:"full_name"`
			Fork     *bool  `json:"fork"`
		} `json:"repository"`
	}
	if json.Unmarshal(data, &event) != nil || event.Repository == nil || event.Repository.Fork == nil {
		return false
	}
	return strings.EqualFold(event.Repository.FullName, repo) && !*event.Repository.Fork
}

// cached answers the record in top's git common directory under conditions.
// The record names repo and came from this resolver's sources. It is younger
// than CacheTTL. Every commit it names is still in the object store.
func (r Resolver) cached(g *gitread.Repo, repo string) (record, bool) {
	path, err := cachePath(g)
	if err != nil {
		return record{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return record{}, false
	}
	var rec record
	if json.Unmarshal(data, &rec) != nil {
		return record{}, false
	}
	if !strings.EqualFold(rec.Repo, repo) || rec.Source != r.source() || time.Since(rec.Checked) > CacheTTL || time.Since(rec.Checked) < 0 {
		return record{}, false
	}
	switch rec.Kind {
	case kindPlain:
		return rec, true
	case kindListed:
		return rec, rec.Upstream != ""
	case kindParent:
		oid, err := gitread.ParseOID(rec.Parent)
		if err != nil || rec.Parent == "" {
			return record{}, false
		}
		if remote, ok := gitread.OpenRemote(rec.ParentURL); ok {
			g.AddAlternate(remote)
		}
		_, err = g.PeelCommit(oid)
		return rec, err == nil
	}
	return record{}, false
}

// cachePath answers where top's record lives.
func cachePath(g *gitread.Repo) (string, error) {
	if g == nil {
		return "", fmt.Errorf("fork scope: git rev-parse --git-common-dir: no work tree")
	}
	return filepath.Join(g.CommonDir(), cacheFile), nil
}

// store writes rec into top's git common directory through a rename. A root in
// no work tree has nowhere to keep it.
func store(g *gitread.Repo, rec record) error {
	if g == nil {
		return nil
	}
	path, err := cachePath(g)
	if err != nil {
		return err
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), cacheFile+".*")
	if err != nil {
		return fmt.Errorf("fork scope: record the fork: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("fork scope: record the fork: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fork scope: record the fork: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("fork scope: record the fork: %w", err)
	}
	return nil
}

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

// fetchParent fetches the parent's branch and answers its commit.
func fetchParent(g *gitread.Repo, parentURL, branch string) (string, error) {
	remote, ok := gitread.OpenRemote(parentURL)
	if !ok {
		return "", fmt.Errorf("fork scope: fetch the parent's %s from %s: the remote is not a local repository", branch, parentURL)
	}
	commit, err := remote.Resolve("refs/heads/" + branch)
	if err != nil {
		if commit, err = remote.Head(); err != nil {
			return "", fmt.Errorf("fork scope: resolve the parent's %s: %w", branch, err)
		}
	}
	commit, err = remote.PeelCommit(commit)
	if err != nil {
		return "", fmt.Errorf("fork scope: resolve the parent's %s: %w", branch, err)
	}
	g.AddAlternate(remote)
	return commit.String(), nil
}

// deepen reads the whole history of a shallow clone from origin.
func deepen(g *gitread.Repo) error {
	if g == nil || !g.IsShallow() {
		return nil
	}
	remote := g.OriginURL()
	src, ok := gitread.OpenRemote(remote)
	if !ok {
		return fmt.Errorf("fork scope: deepen the shallow clone from origin: %s is not a local repository", remote)
	}
	if err := g.AdoptObjects(src); err != nil {
		return fmt.Errorf("fork scope: deepen the shallow clone from origin: %w", err)
	}
	if err := g.Unshallow(); err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	return nil
}

// listedUpstream answers the upstream URL that the fork list at url gives repo,
// an OWNER/NAME pair. A missing list, and a repository the list does not name,
// answer an empty URL.
func listedUpstream(repo, url string) (string, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" {
		return "", fmt.Errorf("fork scope: the repository is %q, which is not OWNER/NAME", repo)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fork scope: GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("fork scope: GET %s: %w", url, err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fork scope: GET %s: %s", url, resp.Status)
	}
	return UpstreamFor(string(body), repo, url)
}

// listName is the shape forklist.schema.json gives a key of the fork list.
var listName = regexp.MustCompile(`^[^/\s]+/[^/\s]+$`)

// UpstreamFor reads a fork list. A JSON object, comments allowed, that maps
// each fork's OWNER/NAME to its upstream URL, the shape forklist.schema.json
// states. A list of any other shape is an error.
func UpstreamFor(list, repo, source string) (string, error) {
	var raw any
	if err := json.Unmarshal(jsonc.ToJSON([]byte(list)), &raw); err != nil {
		return "", fmt.Errorf("fork scope: %s is not a fork list: %w", source, err)
	}
	forks, ok := raw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("fork scope: %s is not a fork list: it is not an object", source)
	}
	found := ""
	for name, value := range forks {
		if !listName.MatchString(name) {
			return "", fmt.Errorf("fork scope: %s is not a fork list: %q is not OWNER/NAME", source, name)
		}
		url, ok := value.(string)
		if !ok || url == "" {
			return "", fmt.Errorf("fork scope: %s is not a fork list: %q maps to no URL", source, name)
		}
		if strings.EqualFold(name, repo) {
			found = url
		}
	}
	return found, nil
}

// upstreamTags answers the commit each tag of upstream names. An upstream with
// no tags answers none.
func upstreamTags(upstream string) ([]string, error) {
	commits, err := gitread.Tags(upstream)
	if err != nil {
		return nil, fmt.Errorf("fork scope: list the tags of %s: %w", upstream, err)
	}
	out := make([]string, 0, len(commits))
	for _, commit := range commits {
		out = append(out, commit.String())
	}
	return set.Of(out...).Values(), nil
}
