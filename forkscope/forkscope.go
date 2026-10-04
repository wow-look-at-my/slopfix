// Package forkscope tells which lines of a fork's work tree the fork wrote.
//
// A fork carries its parent's whole tree, and the parent never agreed to these
// rules. The repository is the one the work tree's origin names on the GitHub
// server. With no such origin, it is GITHUB_REPOSITORY, for a root inside
// GITHUB_WORKSPACE. A repository that the org's fork list names is measured
// from the newest upstream tag that HEAD contains. Otherwise the API says
// whether the repository is a fork, and the base is the merge base with the
// parent's default branch. Only a line added or changed since that base, or a
// line of a new or untracked file, is the fork's.
package forkscope

import (
	"bufio"
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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/tidwall/jsonc"
	"github.com/wow-look-at-my/go-containers/set"
)

// DefaultGitHubAPI is the API root when GITHUB_API_URL is unset.
const DefaultGitHubAPI = "https://api.github.com"

// DefaultServer is the GitHub server when GITHUB_SERVER_URL is unset.
const DefaultServer = "https://github.com"

// ListURL serves the fork list that the org's .github repository publishes to buildhost.
const ListURL = "https://sites.pazer.build/github/fork-of.json"

// CacheTTL is how long a work tree keeps what the network said about its fork.
const CacheTTL = time.Hour

// cacheFile is the name, in the git common directory, of that record.
const cacheFile = "slopfix-fork.json"

// Resolver finds a fork's base. The zero Resolver reads the process
// environment and the org's fork list.
type Resolver struct {
	Getenv  func(string) string
	ListURL string
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
	repo := ""
	if topErr == nil {
		repo = r.originRepo(top)
	}
	if repo == "" && r.workspaceHolds(root) {
		repo = r.getenv("GITHUB_REPOSITORY")
	}
	if repo == "" {
		return nil, nil
	}
	rec, err := r.record(top, topErr, repo)
	if err != nil {
		return nil, err
	}
	switch rec.Kind {
	case kindListed:
		commit, err := tagBase(top, rec.Upstream, rec.Tags)
		if err != nil {
			return nil, err
		}
		return &Base{top: top, commit: commit}, nil
	case kindParent:
		if err := deepen(top); err != nil {
			return nil, err
		}
		out, err := gitIn(top, "merge-base", "HEAD", rec.Parent)
		if err != nil {
			return nil, fmt.Errorf("fork scope: no merge base between HEAD and the parent's %s (%s) from %s: %w", rec.Branch, rec.Parent, rec.ParentURL, err)
		}
		return &Base{top: top, commit: strings.TrimSpace(out)}, nil
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
	out, err := gitIn(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// originRepo answers the OWNER/NAME the origin remote of top names on the
// GitHub server, or "" for an origin elsewhere or none.
func (r Resolver) originRepo(top string) string {
	out, err := gitIn(top, "config", "--get", "remote.origin.url")
	if err != nil {
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
		if info.Fork {
			if info.Parent == nil || info.Parent.CloneURL == "" || info.Parent.DefaultBranch == "" {
				return identity{}, fmt.Errorf("fork scope: %s is a fork, but the API names no parent clone URL and default branch", repo)
			}
			id = identity{kind: kindParent, parentURL: info.Parent.CloneURL, branch: info.Parent.DefaultBranch}
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

// cached answers the record in top's git common directory, when it names repo,
// came from this resolver's sources, is younger than CacheTTL, and every
// commit it names is still in the object store.
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
		return rec, rec.Upstream != "" && len(rec.Tags) > 0
	case kindParent:
		_, err := gitIn(top, "cat-file", "-e", rec.Parent+"^{commit}")
		return rec, err == nil && rec.Parent != ""
	}
	return record{}, false
}

// cachePath answers where top's record lives.
func cachePath(top string) (string, error) {
	out, err := gitIn(top, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	dir := strings.TrimSpace(out)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(top, dir)
	}
	return filepath.Join(dir, cacheFile), nil
}

// store writes rec into top's git common directory through a rename. A root in
// no work tree has nowhere to keep it.
func store(top string, rec record) error {
	if top == "" {
		return nil
	}
	path, err := cachePath(top)
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

// forkRepo is the part of GET /repos/{repo} that names a parent.
type forkRepo struct {
	Fork   bool `json:"fork"`
	Parent *struct {
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
	} `json:"parent"`
}

// fetchRepo reads GET /repos/{repo}, with GITHUB_TOKEN as the bearer when it is set.
func (r Resolver) fetchRepo(repo string) (forkRepo, error) {
	url := r.api() + "/repos/" + repo
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return forkRepo{}, fmt.Errorf("fork scope: GET %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := r.getenv("GITHUB_TOKEN"); token != "" {
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
	if resp.StatusCode != http.StatusOK {
		return forkRepo{}, fmt.Errorf("fork scope: GET %s: %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}
	var info forkRepo
	if err := json.Unmarshal(body, &info); err != nil {
		return forkRepo{}, fmt.Errorf("fork scope: GET %s: %w", url, err)
	}
	return info, nil
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

// fetchParent fetches the parent's branch into top and answers its commit. The
// fetch leaves blobs behind, so only commits and trees cross the wire.
func fetchParent(top, parentURL, branch string) (string, error) {
	if _, err := gitIn(top, "fetch", "--quiet", "--filter=blob:none", "--no-tags", parentURL, "refs/heads/"+branch); err != nil {
		return "", fmt.Errorf("fork scope: fetch the parent's %s from %s: %w", branch, parentURL, err)
	}
	out, err := gitIn(top, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", fmt.Errorf("fork scope: resolve the parent's %s: %w", branch, err)
	}
	return strings.TrimSpace(out), nil
}

// deepen fetches the whole history of a shallow clone from origin, without blobs.
func deepen(top string) error {
	shallow, err := gitIn(top, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return fmt.Errorf("fork scope: %w", err)
	}
	if strings.TrimSpace(shallow) != "true" {
		return nil
	}
	if _, err := gitIn(top, "fetch", "--quiet", "--unshallow", "--filter=blob:none", "--no-tags", "origin"); err != nil {
		return fmt.Errorf("fork scope: deepen the shallow clone from origin: %w", err)
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

// UpstreamFor reads a fork list: a JSON object, comments allowed, that maps
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

// upstreamTags answers the commit each tag of upstream names.
func upstreamTags(upstream string) ([]string, error) {
	out, err := gitIn(".", "ls-remote", "--tags", upstream)
	if err != nil {
		return nil, fmt.Errorf("fork scope: list the tags of %s: %w", upstream, err)
	}
	direct := map[string]string{}
	peeled := map[string]string{}
	for line := range strings.SplitSeq(out, "\n") {
		sha, ref, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		if name, isPeel := strings.CutSuffix(ref, "^{}"); isPeel {
			peeled[name] = sha
		} else {
			direct[ref] = sha
		}
	}
	commits := set.New[string]()
	for ref, sha := range direct {
		if commit, ok := peeled[ref]; ok {
			sha = commit
		}
		commits.Add(sha)
	}
	if commits.Len() == 0 {
		return nil, fmt.Errorf("fork scope: %s has no tags", upstream)
	}
	return commits.Values(), nil
}

// tagBase answers the newest commit of HEAD's history that an upstream tag
// names. A fork merges upstream releases, so that tag is the upstream it carries.
func tagBase(top, upstream string, tags []string) (string, error) {
	if err := deepen(top); err != nil {
		return "", err
	}
	commits := set.Of(tags...)
	revs, err := gitIn(top, "rev-list", "HEAD")
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	for rev := range strings.SplitSeq(revs, "\n") {
		if commits.Contains(rev) {
			return rev, nil
		}
	}
	return "", fmt.Errorf("fork scope: HEAD contains none of the tags of %s", upstream)
}

// Lines is the part of a fork's work tree that the fork wrote.
type Lines struct {
	// top is the work tree root, with every symlink resolved.
	top string
	// whole holds each file new since the base, or untracked.
	whole set.Set[string]
	// lines holds, for each file the diff touched, the line numbers it added or changed.
	lines map[string]set.Set[int]
}

// Lines answers the lines of the work tree that differ from the base.
func (b *Base) Lines() (*Lines, error) {
	resolved, err := filepath.EvalSymlinks(b.top)
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	own := &Lines{top: resolved, whole: set.New[string](), lines: map[string]set.Set[int]{}}
	diff, err := gitIn(b.top, "-c", "core.quotePath=false", "diff", "-U0", "--no-renames", "--no-color", "--no-ext-diff",
		"--src-prefix=a/", "--dst-prefix=b/", b.commit, "--")
	if err != nil {
		return nil, fmt.Errorf("fork scope: diff against the base %s: %w", b.commit, err)
	}
	if err := own.readDiff(diff); err != nil {
		return nil, err
	}
	untracked, err := gitIn(b.top, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	for name := range strings.SplitSeq(untracked, "\x00") {
		if name != "" {
			own.whole.Add(filepath.ToSlash(name))
		}
	}
	return own, nil
}

// File answers the lines of text that the fork wrote, for text headed for path
// in the work tree. A path the base does not hold is the fork's whole.
func (b *Base) File(path, text string) (*Scope, error) {
	rel, err := relTo(b.top, path)
	if err != nil {
		return nil, err
	}
	listed, err := gitIn(b.top, "ls-tree", "-z", "--full-name", b.commit, "--", rel)
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	if listed == "" {
		return Whole(), nil
	}
	upstream, err := gitIn(b.top, "cat-file", "blob", b.commit+":"+rel)
	if err != nil {
		return nil, fmt.Errorf("fork scope: read %s at the base: %w", rel, err)
	}
	return Changed(upstream, text), nil
}

// relTo answers path relative to the work tree at top, in slash form. A path
// outside it is an error.
func relTo(top, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	dir, name := filepath.Split(abs)
	abs = filepath.Join(realPath(filepath.Clean(dir)), name)
	rel, err := filepath.Rel(realPath(top), abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("fork scope: %s is outside the work tree %s", path, top)
	}
	return filepath.ToSlash(rel), nil
}

// Changed answers the lines of after that differ from before.
func Changed(before, after string) *Scope {
	s := &Scope{}
	for _, op := range opcodes(before, after) {
		if op.Tag != 'r' && op.Tag != 'i' {
			continue
		}
		for j := op.J1; j < op.J2; j++ {
			s.lines.Add(j + 1)
		}
	}
	return s
}

// readDiff records what a zero-context diff added. A hunk header gives the
// count of lines on each side, so a content line is never read as a header.
func (o *Lines) readDiff(diff string) error {
	scanner := bufio.NewScanner(strings.NewReader(diff))
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	var file string
	created := false
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			file, created = "", false
		case line == "--- /dev/null":
			created = true
		case strings.HasPrefix(line, "+++ "):
			name, err := diffPath(strings.TrimPrefix(line, "+++ "))
			if err != nil {
				return err
			}
			file = name
			if file != "" && created {
				o.whole.Add(file)
			}
		case strings.HasPrefix(line, "Binary files ") && strings.HasSuffix(line, " differ"):
			name, err := binaryDiffPath(strings.TrimSuffix(strings.TrimPrefix(line, "Binary files "), " differ"))
			if err != nil {
				return err
			}
			if name != "" {
				o.whole.Add(name)
			}
		case strings.HasPrefix(line, "@@ "):
			start, count, err := hunkNew(line)
			if err != nil {
				return err
			}
			if err := o.addHunk(scanner, file, start, count, hunkOldCount(line)); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// addHunk records a hunk's new lines and reads past its body.
func (o *Lines) addHunk(scanner *bufio.Scanner, file string, start, added, removed int) error {
	if file != "" && added > 0 {
		lines, ok := o.lines[file]
		if !ok {
			lines = set.New[int]()
			o.lines[file] = lines
		}
		for n := start; n < start+added; n++ {
			lines.Add(n)
		}
	}
	for added > 0 || removed > 0 {
		if !scanner.Scan() {
			return fmt.Errorf("fork scope: the diff of %s ends inside a hunk", file)
		}
		switch body := scanner.Text(); {
		case strings.HasPrefix(body, "+"):
			added--
		case strings.HasPrefix(body, "-"):
			removed--
		}
	}
	return nil
}

// diffPath answers the path a "+++" header names, or "" for /dev/null.
func diffPath(field string) (string, error) {
	if field == "/dev/null" {
		return "", nil
	}
	if strings.HasPrefix(field, `"`) {
		unquoted, err := strconv.Unquote(field)
		if err != nil {
			return "", fmt.Errorf("fork scope: the diff names %s, which does not unquote: %w", field, err)
		}
		field = unquoted
	}
	return strings.TrimPrefix(field, "b/"), nil
}

// binaryDiffPath answers the new path of a binary diff's "A and B" pair, or ""
// for a deleted file. The diff runs with --no-renames, so A and B name one path
// unless a side is /dev/null. The halves therefore split at the middle " and ".
func binaryDiffPath(pair string) (string, error) {
	if newSide, ok := strings.CutPrefix(pair, "/dev/null and "); ok {
		return diffPath(newSide)
	}
	if strings.HasSuffix(pair, " and /dev/null") {
		return "", nil
	}
	refused := fmt.Errorf("fork scope: the binary diff %q does not name one path on each side", pair)
	half := (len(pair) - len(" and ")) / 2
	if half <= 0 || pair[half:half+len(" and ")] != " and " {
		return "", refused
	}
	oldSide, err := diffPath(strings.Replace(pair[:half], "a/", "b/", 1))
	if err != nil {
		return "", err
	}
	newSide, err := diffPath(pair[half+len(" and "):])
	if err != nil {
		return "", err
	}
	if oldSide != newSide {
		return "", refused
	}
	return newSide, nil
}

// hunkRange reads "start[,count]" after the sign of a hunk header side.
func hunkRange(side string) (start, count int, err error) {
	first, rest, ranged := strings.Cut(side, ",")
	if start, err = strconv.Atoi(first); err != nil {
		return 0, 0, err
	}
	if !ranged {
		return start, 1, nil
	}
	count, err = strconv.Atoi(rest)
	return start, count, err
}

// hunkNew answers the first new line and the new line count of "@@ -a,b +c,d @@".
func hunkNew(header string) (start, count int, err error) {
	fields := strings.Fields(header)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "+") {
		return 0, 0, fmt.Errorf("fork scope: %q is not a hunk header", header)
	}
	start, count, err = hunkRange(strings.TrimPrefix(fields[2], "+"))
	if err != nil {
		return 0, 0, fmt.Errorf("fork scope: %q is not a hunk header: %w", header, err)
	}
	return start, count, nil
}

// hunkOldCount answers the line count of a header hunkNew already read.
func hunkOldCount(header string) int {
	_, count, _ := hunkRange(strings.TrimPrefix(strings.Fields(header)[1], "-"))
	return count
}

// rel answers path relative to the work tree, in slash form.
func (o *Lines) rel(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	rel, err := filepath.Rel(o.top, abs)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// Scope answers the lines of path that the fork wrote. A file the fork never
// touched answers an empty Scope.
func (o *Lines) Scope(path string) *Scope {
	name := o.rel(path)
	if o.whole.Contains(name) {
		return Whole()
	}
	return &Scope{lines: o.lines[name]}
}

// Whole reports whether the fork wrote all of path: it is new since the base, or untracked.
func (o *Lines) Whole(path string) bool {
	return o.whole.Contains(o.rel(path))
}

// Holds reports whether the fork wrote any line from first to last of path.
func (o *Lines) Holds(path string, first, last int) bool {
	return o.Scope(path).Holds(first, last)
}

// Scope is the lines of one file that the fork wrote, counted from one.
type Scope struct {
	whole bool
	lines set.Set[int]
}

// Whole is the Scope of a file the fork wrote all of.
func Whole() *Scope { return &Scope{whole: true} }

// OfLines is the Scope of the named lines.
func OfLines(lines ...int) *Scope { return &Scope{lines: set.Of(lines...)} }

// All reports whether the fork wrote every line.
func (s *Scope) All() bool { return s.whole }

// Empty reports whether the fork wrote no line.
func (s *Scope) Empty() bool { return !s.whole && s.lines.Len() == 0 }

// Owns reports whether the fork wrote line n.
func (s *Scope) Owns(n int) bool { return s.whole || s.lines.Contains(n) }

// Holds reports whether the fork wrote any line from first to last. A line
// number below one stands for the file, which the fork wrote when it wrote any
// line of it.
func (s *Scope) Holds(first, last int) bool {
	if s.whole {
		return true
	}
	if first < 1 {
		return s.lines.Len() > 0
	}
	for n := first; n <= max(first, last); n++ {
		if s.lines.Contains(n) {
			return true
		}
	}
	return false
}

// Keep answers after with every change to a line the fork did not write put
// back as before had it. A change is a run of lines a line diff pairs up. It
// lands when the fork wrote every line it replaces. A run of new lines between
// old ones lands when the fork wrote a line beside it.
func Keep(before, after string, s *Scope) string {
	if s.whole || before == after {
		return after
	}
	a, b := splitLines(before), splitLines(after)
	var out strings.Builder
	for _, op := range opcodes(before, after) {
		if op.Tag == 'e' || !s.mayChange(op.I1, op.I2) {
			out.WriteString(strings.Join(a[op.I1:op.I2], ""))
			continue
		}
		out.WriteString(strings.Join(b[op.J1:op.J2], ""))
	}
	return out.String()
}

// mayChange reports whether the fork wrote the lines from i1 up to i2,
// counted from zero.
func (s *Scope) mayChange(i1, i2 int) bool {
	if i1 == i2 {
		return s.Owns(i1) || s.Owns(i1+1)
	}
	for i := i1; i < i2; i++ {
		if !s.Owns(i + 1) {
			return false
		}
	}
	return true
}

// Carry answers the lines of after that the fork wrote, where s names those of
// before. A line after keeps from before keeps its owner. A line after changed
// is the fork's.
func Carry(before, after string, s *Scope) *Scope {
	if s.whole {
		return Whole()
	}
	out := &Scope{}
	for _, op := range opcodes(before, after) {
		for j := op.J1; j < op.J2; j++ {
			if op.Tag != 'e' || s.Owns(op.I1+j-op.J1+1) {
				out.lines.Add(j + 1)
			}
		}
	}
	return out
}

// Removed answers the text of every line before had that after replaced or dropped.
func Removed(before, after string) string {
	a := splitLines(before)
	var out strings.Builder
	for _, op := range opcodes(before, after) {
		if op.Tag == 'r' || op.Tag == 'd' {
			out.WriteString(strings.Join(a[op.I1:op.I2], ""))
		}
	}
	return out.String()
}

// opcodes answers the line diff from before to after.
func opcodes(before, after string) []difflib.OpCode {
	return difflib.NewMatcherWithJunk(splitLines(before), splitLines(after), false, nil).GetOpCodes()
}

// splitLines cuts s after each line end. The pieces join back to s.
func splitLines(s string) []string {
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
