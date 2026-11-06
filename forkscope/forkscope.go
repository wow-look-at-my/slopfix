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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	top, topErr := toplevel(root)
	repo := r.repoName(root, top, topErr)
	if repo == "" {
		return nil, nil
	}
	rec, err := r.record(top, topErr, repo)
	if err != nil {
		return nil, err
	}
	switch rec.Kind {
	case kindListed:
		commit, err := tagBase(top, rec.Tags)
		if err != nil {
			return nil, err
		}
		if commit == "" {
			if commit, err = snapshotBase(top, &rec); err != nil {
				return nil, err
			}
		}
		return &Base{top: top, commit: commit, upstream: rec.Tip}, nil
	case kindParent:
		base, err := parentBase(top, rec)
		if err != nil {
			return nil, fmt.Errorf("fork scope: no merge base between HEAD and the parent's %s (%s) from %s: %w", rec.Branch, rec.Parent, rec.ParentURL, err)
		}
		return &Base{top: top, commit: base}, nil
	}
	return nil, nil
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

// toplevel answers the work tree root that holds dir.
func toplevel(dir string) (string, error) {
	repo, err := openTop(dir)
	if err != nil {
		return "", err
	}
	return repo.WorkTree(), nil
}

// originRepo answers the OWNER/NAME the origin remote of top names on the
// GitHub server, or "" for an origin elsewhere or none.
func (r Resolver) originRepo(top string) string {
	repo, err := openTop(top)
	if err != nil {
		return ""
	}
	out := repo.OriginURL()
	if out == "" {
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
	return repoOfURL(strings.TrimSpace(out), serverURL.Hostname())
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
func (r Resolver) record(top string, topErr error, repo string) (record, error) {
	if topErr == nil {
		if rec, ok := r.cached(top, repo); ok {
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
		if rec.Parent, err = fetchParent(top, id.parentURL, id.branch); err != nil {
			return record{}, err
		}
	}
	if err := store(top, rec); err != nil {
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
func (r Resolver) cached(top, repo string) (record, bool) {
	path, err := cachePath(top)
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
		return rec, rec.Parent != "" && hasCommit(top, rec.Parent)
	}
	return record{}, false
}

// cachePath answers where top's record lives.
func cachePath(top string) (string, error) {
	return recordPath(top, cacheFile)
}

// recordPath answers where the record name lives in top's git common directory.
func recordPath(top, name string) (string, error) {
	repo, err := openTop(top)
	if err != nil {
		return "", err
	}
	return filepath.Join(repo.CommonDir(), name), nil
}

// store writes rec into top's git common directory through a rename. A root in
// no work tree has nowhere to keep it.
func store(top string, rec record) error {
	if top == "" {
		return nil
	}
	return writeRecord(top, cacheFile, rec)
}

// gitIn runs git in dir and answers its output, or an error that quotes git's stderr.
func gitIn(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exit.Stderr)))
	}
	return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

// deepen fetches the whole history of a shallow clone from origin, without blobs.
func deepen(top string) error {
	repo, err := openTop(top)
	if err != nil {
		return err
	}
	if !repo.IsShallow() {
		return nil
	}
	if _, err := gitIn(top, "fetch", "--quiet", "--unshallow", "--filter=blob:none", "--no-tags", "origin"); err != nil {
		return fmt.Errorf("fork scope: deepen the shallow clone from origin: %w", err)
	}
	return nil
}
