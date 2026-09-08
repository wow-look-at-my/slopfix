// linkify.go turns a reference into the URL a reader can open.
//
// The rule it serves is not "put a link on everything". A link is a demand to
// stop reading and move your hand, and the reader pays that cost before they
// know whether it was worth paying. So a reference whose target cannot be shown
// to exist is left as plain text. Silence is the correct answer there, never a
// guess at a URL.
//
// Everything here runs in the render path, so each git call is bounded and only
// made when a token actually needs it.
package linkrefs

import (
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
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

// finished reports the states where the page has nothing left to do: the words stay plain and the dot carries the link.
func finished(s PullState) bool { return s == StateMerged || s == StateClosed }

// pullState asks about the reference behind a match, and answers StateUnknown for anything not a pull request.
func pullState(ref Ref, res Resolver) PullState {
	var repo Repo
	var number string
	switch ref.Kind {
	case "an issue or pull request number":
		owner, name, n := splitNumber(ref.Text)
		repo, number = Repo{Owner: owner, Name: name}, n
	case "a bare GitHub URL":
		r, n, ok := IssueRef(ref.Text)
		if !ok {
			return StateUnknown
		}
		repo, number = r, n
	default:
		return StateUnknown
	}
	if !repo.valid() || number == "" {
		return StateUnknown
	}
	return res.PullState(repo, number)
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

// statusJQ flattens the rollup gh returns into the few words the decision needs.
const statusJQ = `{state:.state,mergeable:.mergeable,mergeState:.mergeStateStatus,` +
	`checks:[.statusCheckRollup[]?|if .__typename=="CheckRun" then ` +
	`(if .status!="COMPLETED" then "PENDING" else (.conclusion//"") end) else (.state//"") end]}`

// pullView is statusJQ's output.
type pullView struct {
	State      string   `json:"state"`
	Mergeable  string   `json:"mergeable"`
	MergeState string   `json:"mergeState"`
	Checks     []string `json:"checks"`
}

// PullState asks GitHub what a pull request is doing, memoized per reference.
// Every failure answers StateUnknown, so a lookup that could not answer never shows as green.
func (g *GitResolver) PullState(repo Repo, number string) PullState {
	if !repo.valid() || number == "" {
		return StateUnknown
	}
	key := repo.Owner + "/" + repo.Name + "#" + number

	g.prMu.Lock()
	defer g.prMu.Unlock()
	if g.prSeen == nil {
		g.prSeen = map[string]PullState{}
	}
	if s, ok := g.prSeen[key]; ok {
		return s
	}

	ctx, cancel := context.WithTimeout(context.Background(), ghTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", "pr", "view", number,
		"-R", repo.Owner+"/"+repo.Name,
		"--json", "state,mergeable,mergeStateStatus,statusCheckRollup",
		"--jq", statusJQ).Output()

	state := StateUnknown
	if err == nil {
		var v pullView
		if json.Unmarshal(out, &v) == nil {
			state = classify(v)
		}
	}
	g.prSeen[key] = state
	return state
}

// classify turns a pull request's fields into the dot it earns.
//
// Order is severity: a merged or closed pull request is finished whatever its
// checks say, a failing check is a defect, a conflict is a merge-time chore,
// and a run still going is not yet news. Green is what is left over.
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
