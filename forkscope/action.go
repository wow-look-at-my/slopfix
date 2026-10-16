package forkscope

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

// actionFile is the name, in the git common directory, of the action record.
const actionFile = "slopfix-actions.json"

// actionRecord is what the network said about the runs.using of one action at one ref.
type actionRecord struct {
	Source  string    `json:"source"`
	Checked time.Time `json:"checked"`
	Using   string    `json:"using"`
}

// actions remembers each answer for the life of the process. The mutex also
// keeps the record file to one writer.
var actions = struct {
	sync.Mutex
	byKey map[string]string
}{byKey: map[string]string{}}

// ActionUsing answers the runs.using of the action that the step value uses
// names, as OWNER/REPO[/PATH]@REF. The work tree that holds root keeps each
// answer, so a walk asks once for each action at each ref. A value that cannot
// be read is an error.
func (r Resolver) ActionUsing(root, uses string) (string, error) {
	repo, path, ref, ok := splitUses(uses)
	if !ok {
		return "", fmt.Errorf("action %q: not OWNER/REPO[/PATH]@REF", uses)
	}
	key := r.api() + " " + uses
	actions.Lock()
	defer actions.Unlock()
	if using, ok := actions.byKey[key]; ok {
		return using, nil
	}
	top, topErr := toplevel(root)
	if topErr == nil {
		if using, ok := r.cachedAction(top, uses); ok {
			actions.byKey[key] = using
			return using, nil
		}
	}
	using, err := r.fetchUsing(repo, path, ref)
	if err != nil {
		return "", err
	}
	if topErr == nil {
		if err := r.recordAction(top, uses, using); err != nil {
			return "", err
		}
	}
	actions.byKey[key] = using
	return using, nil
}

// RecordAction writes what the network would answer about uses into the work
// tree at top. A fixture states the answer this way.
func (r Resolver) RecordAction(top, uses, using string) error {
	actions.Lock()
	defer actions.Unlock()
	return r.recordAction(top, uses, using)
}

func (r Resolver) recordAction(top, uses, using string) error {
	recs := readActions(top)
	recs[uses] = actionRecord{Source: r.api(), Checked: time.Now(), Using: using}
	return writeRecord(top, actionFile, recs)
}

// cachedAction answers the recorded runs.using of uses when the record came
// from this resolver's API and is younger than CacheTTL.
func (r Resolver) cachedAction(top, uses string) (string, bool) {
	rec, ok := readActions(top)[uses]
	if !ok {
		return "", false
	}
	age := time.Since(rec.Checked)
	if rec.Source != r.api() || age > CacheTTL || age < 0 {
		return "", false
	}
	return rec.Using, true
}

// readActions answers the action records of top. A missing or bad file holds none.
func readActions(top string) map[string]actionRecord {
	recs := map[string]actionRecord{}
	path, err := recordPath(top, actionFile)
	if err != nil {
		return recs
	}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &recs) != nil {
		return map[string]actionRecord{}
	}
	return recs
}

// splitUses cuts OWNER/REPO[/PATH]@REF into its parts.
func splitUses(uses string) (repo, path, ref string, ok bool) {
	name, ref, found := strings.Cut(uses, "@")
	parts := strings.SplitN(name, "/", 3)
	if !found || ref == "" || len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	repo = parts[0] + "/" + parts[1]
	if len(parts) == 3 {
		path = strings.Trim(parts[2], "/")
	}
	return repo, path, ref, true
}

// fetchUsing reads action.yml, then action.yaml, at ref, and answers its runs.using.
func (r Resolver) fetchUsing(repo, path, ref string) (string, error) {
	var firstErr error
	for _, name := range []string{"action.yml", "action.yaml"} {
		file := name
		if path != "" {
			file = path + "/" + name
		}
		body, err := r.fetchFile(repo, file, ref)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		var meta struct {
			Runs struct {
				Using string `yaml:"using"`
			} `yaml:"runs"`
		}
		if err := yaml.Unmarshal(body, &meta); err != nil {
			return "", fmt.Errorf("action %s/%s@%s: %w", repo, file, ref, err)
		}
		if meta.Runs.Using == "" {
			return "", fmt.Errorf("action %s/%s@%s: no runs.using", repo, file, ref)
		}
		return meta.Runs.Using, nil
	}
	return "", firstErr
}

// branches remembers each repository's default branch for the life of the process.
var branches = struct {
	sync.Mutex
	byKey map[string]string
}{byKey: map[string]string{}}

// DefaultBranch answers the default branch of repo, as OWNER/REPO.
func (r Resolver) DefaultBranch(repo string) (string, error) {
	key := r.api() + " " + repo
	branches.Lock()
	defer branches.Unlock()
	if branch, ok := branches.byKey[key]; ok {
		return branch, nil
	}
	body, err := r.get("repos/"+repo, "application/vnd.github+json")
	if err != nil {
		return "", err
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", fmt.Errorf("repository %s: %w", repo, err)
	}
	if info.DefaultBranch == "" {
		return "", fmt.Errorf("repository %s: no default branch", repo)
	}
	branches.byKey[key] = info.DefaultBranch
	return info.DefaultBranch, nil
}

// fetchFile reads one file of repo at ref through the contents API.
func (r Resolver) fetchFile(repo, file, ref string) ([]byte, error) {
	return r.get("repos/"+repo+"/contents/"+escapePath(file)+"?ref="+url.QueryEscape(ref), "application/vnd.github.raw")
}

// get reads one API endpoint. It asks as fetchRepo does: through the gh CLI
// with no token and the default API.
func (r Resolver) get(endpoint, accept string) ([]byte, error) {
	if r.token() == "" && r.getenv("GITHUB_API_URL") == "" {
		run := r.GH
		if run == nil {
			run = runGH
		}
		out, err := run("api", "-H", "Accept: "+accept, endpoint)
		if err != nil {
			return nil, fmt.Errorf("gh api %s: %w", endpoint, err)
		}
		return out, nil
	}
	full := r.api() + "/" + endpoint
	req, err := http.NewRequest(http.MethodGet, full, nil)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", full, err)
	}
	req.Header.Set("Accept", accept)
	if token := r.token(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", full, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", full, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", full, resp.Status)
	}
	return body, nil
}

// escapePath escapes each segment of a slash path.
func escapePath(path string) string {
	segs := strings.Split(path, "/")
	for idx, seg := range segs {
		segs[idx] = url.PathEscape(seg)
	}
	return strings.Join(segs, "/")
}
