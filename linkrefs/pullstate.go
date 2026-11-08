// pullstate.go asks GitHub for the state of a pull request and caches the answer.

package linkrefs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wow-look-at-my/go-containers/set"
)

// ghTimeout bounds the call that leaves the machine, longer than gitTimeout because a remote answer is not local.
const ghTimeout = 2 * time.Second

// checksTimeout is longer than ghTimeout: `gh wait-ci checks` on a private repository takes several seconds.
const checksTimeout = 12 * time.Second

// cacheTTL is how long an open pull request's state is reused across flushes. A finished one never changes.
const cacheTTL = time.Minute

// cachePath is the file that holds a pull request's state. The sweep in run.go collects it.
func cachePath(key string) string {
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(os.TempDir(), "slopfix-linkrefs-pr-"+hex.EncodeToString(sum[:])[:16])
}

func readCachedPull(key string) (PullState, bool) {
	p := cachePath(key)
	info, err := os.Stat(p)
	if err != nil {
		return StateUnknown, false
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return StateUnknown, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return StateUnknown, false
	}
	s := PullState(n)
	if s <= StateUnknown || s > StateMergeable {
		return StateUnknown, false
	}
	if !finished(s) && time.Since(info.ModTime()) > cacheTTL {
		return StateUnknown, false
	}
	return s, true
}

// writeCachedPull keeps an answer. An unknown is not an answer, so it is never kept.
func writeCachedPull(key string, s PullState) {
	if s == StateUnknown {
		return
	}
	os.WriteFile(cachePath(key), []byte(strconv.Itoa(int(s))), 0o600)
}

// pullView is the fields of a pull request the decision reads.
type pullView struct {
	State      string   `json:"state"`
	Mergeable  string   `json:"mergeable"`
	MergeState string   `json:"mergeStateStatus"`
	Checks     []string `json:"-"`
}

// pullKey is the memo key for a pull request.
func pullKey(repo Repo, number string) string { return repo.Owner + "/" + repo.Name + "#" + number }

// PullState asks GitHub what a pull request is doing, memoized per reference.
// Every failure answers StateUnknown, so a lookup that could not answer never shows as green.
func (g *GitResolver) PullState(repo Repo, number string) PullState {
	if !repo.valid() || number == "" {
		return StateUnknown
	}
	key := pullKey(repo, number)
	if s, ok := g.seen(key); ok {
		return s
	}
	s := lookupPull(repo, number)
	g.store(key, s)
	return s
}

// Prefetch asks about every pull request at once, so a flush that names several waits for the slowest one alone.
func (g *GitResolver) Prefetch(refs []PullRef) {
	var wg sync.WaitGroup
	asked := set.New[string]()
	for _, r := range refs {
		key := pullKey(r.Repo, r.Number)
		if !r.Repo.valid() || r.Number == "" || asked.Contains(key) {
			continue
		}
		if _, ok := g.seen(key); ok {
			continue
		}
		asked.Add(key)
		wg.Add(1)
		go func(r PullRef, key string) {
			defer wg.Done()
			g.store(key, lookupPull(r.Repo, r.Number))
		}(r, key)
	}
	wg.Wait()
}

func (g *GitResolver) seen(key string) (PullState, bool) {
	g.prMu.Lock()
	defer g.prMu.Unlock()
	s, ok := g.prSeen[key]
	return s, ok
}

func (g *GitResolver) store(key string, s PullState) {
	g.prMu.Lock()
	defer g.prMu.Unlock()
	if g.prSeen == nil {
		g.prSeen = map[string]PullState{}
	}
	g.prSeen[key] = s
}

// gh runs one bounded gh call and returns its stdout.
func gh(timeout time.Duration, args ...string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", args...).Output()
	return out, err == nil
}

// lookupPull answers from the disk cache when it can, since every flush is a new process.
func lookupPull(repo Repo, number string) PullState {
	key := pullKey(repo, number)
	if s, ok := readCachedPull(key); ok {
		return s
	}
	s := askPull(repo, number)
	writeCachedPull(key, s)
	return s
}

// askPull reads the pull request's own fields, then the checks on its head.
func askPull(repo Repo, number string) PullState {
	slug := repo.Owner + "/" + repo.Name
	out, ok := gh(ghTimeout, "pr", "view", number, "-R", slug, "--json", "state,mergeable,mergeStateStatus,headRefOid")
	if !ok {
		return StateUnknown
	}
	var v struct {
		pullView
		Head string `json:"headRefOid"`
	}
	if json.Unmarshal(out, &v) != nil || v.State == "" {
		return StateUnknown
	}
	if v.State != "OPEN" || v.Head == "" {
		return classify(v.pullView)
	}
	if checks, ok := headChecks(slug, v.Head); ok {
		v.Checks = checks
		return classify(v.pullView)
	}
	v.Checks = mergeStateChecks(v.MergeState)
	return classify(v.pullView)
}

// checksReport is the part of `gh wait-ci checks --json` the decision reads.
type checksReport struct {
	Statuses []struct {
		State string `json:"state"`
	} `json:"statuses"`
	CheckRuns []struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"check_runs"`
}

// headChecks reads every check run and commit status on the head commit, in the words classify reads.
func headChecks(slug, sha string) ([]string, bool) {
	out, ok := gh(checksTimeout, "wait-ci", "checks", "-R", slug, "--sha", sha, "--json")
	if !ok {
		return nil, false
	}
	var r checksReport
	if json.Unmarshal(out, &r) != nil {
		return nil, false
	}
	return r.words(), true
}

func (r checksReport) words() []string {
	var checks []string
	for _, s := range r.Statuses {
		checks = append(checks, strings.ToUpper(s.State))
	}
	for _, c := range r.CheckRuns {
		if !strings.EqualFold(c.Status, "completed") {
			checks = append(checks, "PENDING")
			continue
		}
		checks = append(checks, strings.ToUpper(c.Conclusion))
	}
	return checks
}

// mergeStateChecks stands in for the checks when they cannot be read. GitHub
// computes mergeStateStatus from the same checks, so it still says red or green.
func mergeStateChecks(mergeState string) []string {
	switch mergeState {
	case "CLEAN":
		return nil
	case "UNSTABLE":
		return []string{"FAILURE"}
	}
	return []string{"PENDING"}
}

// classify turns a pull request's fields into the dot it earns.
//
// Order is severity: a merged or closed pull request is finished whatever its
// checks say. A failing check is a defect. A conflict is a merge-time chore,
// and a run still going is not yet news.
func classify(v pullView) PullState {
	switch v.State {
	case "MERGED":
		return StateMerged
	case "CLOSED":
		return StateClosed
	}
	if slices.ContainsFunc(v.Checks, checkFailed) {
		return StateFailing
	}
	if v.Mergeable == "CONFLICTING" || v.MergeState == "DIRTY" {
		return StateConflicted
	}
	if slices.Contains(v.Checks, "PENDING") || slices.Contains(v.Checks, "EXPECTED") {
		return StatePending
	}
	return StateMergeable
}

// checkFailed reports the conclusions that mean a check did not pass. SKIPPED and NEUTRAL are not among them.
func checkFailed(conclusion string) bool {
	switch conclusion {
	case "FAILURE", "ERROR", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE":
		return true
	}
	return false
}

// remoteRe pulls owner and repo out of every spelling of a GitHub remote URL.
var remoteRe = regexp.MustCompile(`github\.com[:/]+([A-Za-z0-9._-]+)/([A-Za-z0-9._-]+?)(?:\.git)?/?$`)

// parseRemote reads owner/repo off a remote URL. Credentials in the URL are discarded, never carried into a link.
func parseRemote(url string) (Repo, bool) {
	m := remoteRe.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil {
		return Repo{}, false
	}
	return Repo{Owner: m[1], Name: m[2]}, true
}
