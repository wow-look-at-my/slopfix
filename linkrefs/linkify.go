// linkify.go turns a reference into the URL a reader can open.
//
// The rule it serves is not "put a link on everything". A link is a demand to
// stop reading and move your hand. The reader pays that cost before they know
// whether it was worth paying. So a reference whose target cannot be shown to
// exist is left as plain text. Silence is the correct answer there, never a
// guess at a URL.
//
// Everything here runs in the render path, so each git call is bounded and
// only made when a token needs it.
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

// gitTimeout bounds a git call, since this runs while a message streams.
const gitTimeout = 300 * time.Millisecond

// Repo is the owner/repo a bare reference resolves against.
type Repo struct {
	Owner string
	Name  string
}

func (r Repo) valid() bool { return r.Owner != "" && r.Name != "" }

func (r Repo) url() string { return "https://github.com/" + r.Owner + "/" + r.Name }

// Resolver answers what a rewrite needs about the checkout, as an interface so tests never shell out to git.
type Resolver interface {
	// Repo is the origin remote's owner and name, when the remote exists.
	Repo() (Repo, bool)
	// BranchExists reports whether the branch is on origin. An unpushed branch has no page to open.
	BranchExists(branch string) bool
	// CommitExists reports whether the object is in this repository.
	CommitExists(sha string) bool
	// DefaultBranch is the base a compare URL is taken against.
	DefaultBranch() string
	// PullState decides the dot; StateUnknown renders no dot, so an unanswered question never shows as green.
	PullState(repo Repo, number string) PullState
}

// PullState is what a pull request is doing right now, rendered as a character beside the reference.
type PullState int

const (
	// StateUnknown is every unanswered question: no `gh`, no network, a timeout, an issue number. It renders no dot.
	StateUnknown PullState = iota
	StateMerged
	StateClosed
	StateFailing
	StateConflicted
	StatePending
	StateMergeable
)

// dot is the character rendered beside the reference.
func dot(s PullState) string {
	switch s {
	case StateMerged:
		return "🟣"
	case StateClosed:
		return "⚫"
	case StateFailing:
		return "🔴"
	case StateConflicted:
		return "🟠"
	case StatePending:
		return "🟡"
	case StateMergeable:
		return "🟢"
	}
	return ""
}

// PullRef names a pull request a flush asks about.
type PullRef struct {
	Repo   Repo
	Number string
}

// prefetcher is a Resolver that can ask about several pull requests at once.
type prefetcher interface {
	Prefetch(refs []PullRef)
}

// isDot reports whether s is one of the dots, so a link that already carries one is not given another.
func isDot(s string) bool {
	for st := StateMerged; st <= StateMergeable; st++ {
		if s == dot(st) {
			return true
		}
	}
	return false
}

// endsWithDot reports whether the text before a link already ends in a dot and its space.
func endsWithDot(before string) bool {
	before = strings.TrimSuffix(before, " ")
	for st := StateMerged; st <= StateMergeable; st++ {
		if strings.HasSuffix(before, dot(st)) {
			return true
		}
	}
	return false
}

// BadgeLink gives a markdown link to a pull request the dot a bare reference
// gets. The link text and URL stay as written, and an unknown state changes nothing.
func BadgeLink(link, text, url string, res Resolver) (string, bool) {
	if isDot(strings.TrimSpace(text)) {
		return "", false
	}
	repo, number, ok := IssueRef(url)
	if !ok {
		return "", false
	}
	state := res.PullState(repo, number)
	switch {
	case state == StateUnknown:
		return "", false
	case finished(state):
		return "[" + dot(state) + "](" + url + ") " + text, true
	default:
		return dot(state) + " " + link, true
	}
}

// pullRefOf names the pull request behind a reference, when it has one.
func pullRefOf(ref Ref) (PullRef, bool) {
	switch ref.Kind {
	case "an issue or pull request number":
		owner, name, n := splitNumber(ref.Text)
		r := PullRef{Repo{Owner: owner, Name: name}, n}
		return r, r.Repo.valid() && n != ""
	case "a bare GitHub URL":
		repo, n, ok := IssueRef(ref.Text)
		return PullRef{repo, n}, ok
	}
	return PullRef{}, false
}

// finished reports the states where the page has nothing left to do: the words stay plain and the dot carries the link.
func finished(s PullState) bool { return s == StateMerged || s == StateClosed }

// pullState asks about the reference behind a match, and answers StateUnknown for anything not a pull request.
func pullState(ref Ref, res Resolver) PullState {
	p, ok := pullRefOf(ref)
	if !ok {
		return StateUnknown
	}
	return res.PullState(p.Repo, p.Number)
}

// Linkify returns the markdown link for a reference, and false when it must be left as written.
func Linkify(ref Ref, res Resolver) (string, bool) {
	url, ok := refURL(ref, res)
	if !ok {
		return "", false
	}
	text := ref.Text
	if ref.Backticked {
		// Backticks go back inside the brackets, keeping the code span and the link as the same span.
		text = "`" + text + "`"
	}

	state := pullState(ref, res)
	switch {
	case state == StateUnknown:
		return "[" + text + "](" + url + ")", true
	case finished(state):
		// The words go back to plain text and the dot carries the link.
		return "[" + dot(state) + "](" + url + ") " + text, true
	default:
		return dot(state) + " [" + text + "](" + url + ")", true
	}
}

func refURL(ref Ref, res Resolver) (string, bool) {
	switch ref.Kind {
	case "a bare GitHub URL":
		// Already a URL, proven to exist by whoever wrote it: no repository and no probe needed.
		return ref.Text, true

	case "an issue or pull request number":
		owner, name, number := splitNumber(ref.Text)
		repo := Repo{Owner: owner, Name: name}
		// A BARE #N is never linked: the repository is a guess, so it would point at a real but unrelated issue.
		if !repo.valid() || number == "" {
			return "", false
		}
		// /issues/N, never /pull/N: GitHub redirects an issue number to its pull request, and /pull/N on an issue fails.
		return repo.url() + "/issues/" + number, true

	case "a commit SHA":
		repo, ok := res.Repo()
		if !ok || !res.CommitExists(ref.Text) {
			return "", false
		}
		return repo.url() + "/commit/" + ref.Text, true

	case "a branch":
		repo, ok := res.Repo()
		if !ok || !res.BranchExists(ref.Text) {
			return "", false
		}
		base := res.DefaultBranch()
		if base == "" {
			return "", false
		}
		return repo.url() + "/compare/" + base + "..." + ref.Text + "?expand=1", true
	}
	return "", false
}

func splitNumber(token string) (owner, name, number string) {
	hash := strings.IndexByte(token, '#')
	if hash < 0 {
		return "", "", ""
	}
	number = token[hash+1:]
	if slug := token[:hash]; slug != "" {
		if slash := strings.IndexByte(slug, '/'); slash > 0 && slash < len(slug)-1 {
			owner, name = slug[:slash], slug[slash+1:]
		}
	}
	return owner, name, number
}

// GitResolver answers from the checkout the session stands in. Every answer is memoized, since this runs per flush.
type GitResolver struct {
	Dir string

	once      sync.Once
	repo      Repo
	repoFound bool
	base      string

	mu    sync.Mutex
	cache map[string]bool

	// A pull request's state is not a yes or no, so it cannot share the memo above.
	prMu   sync.Mutex
	prSeen map[string]PullState
}

func (g *GitResolver) git(args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Dir
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func (g *GitResolver) load() {
	g.once.Do(func() {
		if url, ok := g.git("remote", "get-url", "origin"); ok {
			g.repo, g.repoFound = parseRemote(url)
		}
		// origin/HEAD names the default branch, when the clone recorded it.
		if head, ok := g.git("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); ok {
			g.base = strings.TrimPrefix(head, "origin/")
		}
	})
}

func (g *GitResolver) Repo() (Repo, bool) { g.load(); return g.repo, g.repoFound }

func (g *GitResolver) DefaultBranch() string { g.load(); return g.base }

func (g *GitResolver) memo(key string, probe func() bool) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cache == nil {
		g.cache = map[string]bool{}
	}
	if v, ok := g.cache[key]; ok {
		return v
	}
	v := probe()
	g.cache[key] = v
	return v
}

// BranchExists asks for the remote-tracking ref, not the local branch. A branch
// that exists only locally has no compare page: GitHub cannot show a diff
// against a ref it has never received.
func (g *GitResolver) BranchExists(branch string) bool {
	return g.memo("branch:"+branch, func() bool {
		_, ok := g.git("rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+branch)
		return ok
	})
}

func (g *GitResolver) CommitExists(sha string) bool {
	return g.memo("commit:"+sha, func() bool {
		_, ok := g.git("cat-file", "-e", sha+"^{commit}")
		return ok
	})
}

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
