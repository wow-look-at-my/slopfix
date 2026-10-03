// forkscope.go keeps a fork's check to the lines the fork wrote itself.
//
// A fork carries its parent's whole tree, and the parent never agreed to these
// rules. In a GitHub Actions run, a repository that the org's fork list names
// is checked against the newest upstream tag that HEAD contains. Otherwise
// ForkLines asks the API whether the repository is a fork, and uses the merge
// base with the parent's default branch. Only a line added or changed since
// that base, or a line of a new file, can fail the check.
package slopfix

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wow-look-at-my/go-containers/set"
)

// DefaultGitHubAPI is the API root when GITHUB_API_URL is unset.
const DefaultGitHubAPI = "https://api.github.com"

// ForkListURL serves the fork list that the org's .github repository publishes to buildhost.
const ForkListURL = "https://sites.pazer.build/github/fork-of"

// OwnLines is the part of a fork's work tree that the fork wrote.
type OwnLines struct {
	// top is the work tree root, with every symlink resolved.
	top string
	// whole holds each file new since the merge base, or untracked.
	whole set.Set[string]
	// lines holds, for each file the diff touched, the line numbers it added or changed.
	lines map[string]set.Set[int]
}

// forkRepo is the part of GET /repos/{repo} that names a parent.
type forkRepo struct {
	Fork   bool `json:"fork"`
	Parent *struct {
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
	} `json:"parent"`
}

// ForkLines answers the lines a fork wrote, for the work tree that holds
// root.
func ForkLines(root string, getenv func(string) string) (*OwnLines, error) {
	return forkLines(root, getenv, ForkListURL)
}

// forkLines is ForkLines with the fork list read from listURL. No variable sets the URL, because a workflow sets its own.
// It answers nil when getenv names no repository, or one that is neither listed nor a fork. A fork with no base is an error.
func forkLines(root string, getenv func(string) string, listURL string) (*OwnLines, error) {
	repo := getenv("GITHUB_REPOSITORY")
	if repo == "" {
		return nil, nil
	}
	upstream, err := listedUpstream(repo, listURL)
	if err != nil {
		return nil, err
	}
	if upstream != "" {
		out, err := gitIn(root, "rev-parse", "--show-toplevel")
		if err != nil {
			return nil, fmt.Errorf("fork scope: %w", err)
		}
		top := strings.TrimSpace(out)
		base, err := tagBase(top, upstream)
		if err != nil {
			return nil, err
		}
		return linesSince(top, base)
	}
	info, err := fetchRepo(repo, getenv)
	if err != nil {
		return nil, err
	}
	if !info.Fork {
		return nil, nil
	}
	if info.Parent == nil || info.Parent.CloneURL == "" || info.Parent.DefaultBranch == "" {
		return nil, fmt.Errorf("fork scope: %s is a fork, but the API names no parent clone URL and default branch", repo)
	}
	base, top, err := forkBase(root, info.Parent.CloneURL, info.Parent.DefaultBranch)
	if err != nil {
		return nil, err
	}
	return linesSince(top, base)
}

// fetchRepo reads GET /repos/{repo}, with GITHUB_TOKEN as the bearer when it is set.
func fetchRepo(repo string, getenv func(string) string) (forkRepo, error) {
	api := strings.TrimRight(getenv("GITHUB_API_URL"), "/")
	if api == "" {
		api = DefaultGitHubAPI
	}
	url := api + "/repos/" + repo
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return forkRepo{}, fmt.Errorf("fork scope: GET %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := getenv("GITHUB_TOKEN"); token != "" {
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

// forkBase fetches the parent's branch, deepens a shallow clone, and answers
// the merge base of HEAD with that branch, and the work tree root.
// Both fetches leave blobs behind, so only commits and trees cross the wire.
func forkBase(root, parentURL, branch string) (base, top string, err error) {
	out, err := gitIn(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("fork scope: %w", err)
	}
	top = strings.TrimSpace(out)
	if _, err := gitIn(top, "fetch", "--quiet", "--filter=blob:none", "--no-tags", parentURL, "refs/heads/"+branch); err != nil {
		return "", "", fmt.Errorf("fork scope: fetch the parent's %s from %s: %w", branch, parentURL, err)
	}
	out, err = gitIn(top, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", "", fmt.Errorf("fork scope: resolve the parent's %s: %w", branch, err)
	}
	parent := strings.TrimSpace(out)
	if err := deepen(top); err != nil {
		return "", "", err
	}
	out, err = gitIn(top, "merge-base", "HEAD", parent)
	if err != nil {
		return "", "", fmt.Errorf("fork scope: no merge base between HEAD and the parent's %s (%s) from %s: %w", branch, parent, parentURL, err)
	}
	return strings.TrimSpace(out), top, nil
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
		return "", fmt.Errorf("fork scope: GITHUB_REPOSITORY is %q, which is not OWNER/NAME", repo)
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
	return upstreamFor(string(body), repo, url)
}

// upstreamFor reads a fork list: an "OWNER/NAME upstream-URL" line for each fork,
// with blank lines and # comments ignored. A line it cannot read is an error.
func upstreamFor(list, repo, source string) (string, error) {
	for num, line := range strings.Split(list, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.Count(fields[0], "/") != 1 {
			return "", fmt.Errorf("fork scope: %s line %d is not an OWNER/NAME and an upstream URL: %q", source, num+1, line)
		}
		if strings.EqualFold(fields[0], repo) {
			return fields[1], nil
		}
	}
	return "", nil
}

// tagBase answers the newest commit of HEAD's history that an upstream tag
// names. A fork merges upstream releases, so that tag is the upstream it carries.
func tagBase(top, upstream string) (string, error) {
	if err := deepen(top); err != nil {
		return "", err
	}
	out, err := gitIn(top, "ls-remote", "--tags", upstream)
	if err != nil {
		return "", fmt.Errorf("fork scope: list the tags of %s: %w", upstream, err)
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
		return "", fmt.Errorf("fork scope: %s has no tags", upstream)
	}
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

// linesSince answers the lines of the work tree at top that differ from base.
func linesSince(top, base string) (*OwnLines, error) {
	resolved, err := filepath.EvalSymlinks(top)
	if err != nil {
		return nil, fmt.Errorf("fork scope: %w", err)
	}
	own := &OwnLines{top: resolved, whole: set.New[string](), lines: map[string]set.Set[int]{}}
	diff, err := gitIn(top, "-c", "core.quotePath=false", "diff", "-U0", "--no-renames", "--no-color", "--no-ext-diff",
		"--src-prefix=a/", "--dst-prefix=b/", base, "--")
	if err != nil {
		return nil, fmt.Errorf("fork scope: diff against the merge base %s: %w", base, err)
	}
	if err := own.readDiff(diff); err != nil {
		return nil, err
	}
	untracked, err := gitIn(top, "ls-files", "-z", "--others", "--exclude-standard")
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

// readDiff records what a zero-context diff added. A hunk header gives the
// count of lines on each side, so a content line is never read as a header.
func (o *OwnLines) readDiff(diff string) error {
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
func (o *OwnLines) addHunk(scanner *bufio.Scanner, file string, start, added, removed int) error {
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
func (o *OwnLines) rel(path string) string {
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

// Holds reports whether the fork wrote any line from first to last of path.
// A line number below one stands for the file, which the fork wrote when it
// changed any line of it.
func (o *OwnLines) Holds(path string, first, last int) bool {
	name := o.rel(path)
	if o.whole.Contains(name) {
		return true
	}
	lines, ok := o.lines[name]
	if !ok {
		return false
	}
	if first < 1 {
		return true
	}
	for n := first; n <= max(first, last); n++ {
		if lines.Contains(n) {
			return true
		}
	}
	return false
}

// Within keeps what a tree run from root found on lines the fork wrote. A
// repository rule names its file relative to root. Every fixture expectation
// stays, because a fixture is the repository's own test.
func (t TreeRepair) Within(own *OwnLines, root string) TreeRepair {
	if own == nil {
		return t
	}
	findings := t.Findings[:0:0]
	for _, f := range t.Findings {
		path := f.Path
		if RepoIDs.Contains(f.ID) && !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if own.Holds(path, f.Line, f.EndLine) {
			findings = append(findings, f)
		}
	}
	kept := t.Kept[:0:0]
	for _, k := range t.Kept {
		if own.Holds(k.Path, k.LineNo, k.LineNo) {
			kept = append(kept, k)
		}
	}
	t.Findings, t.Kept = findings, kept
	return t
}
