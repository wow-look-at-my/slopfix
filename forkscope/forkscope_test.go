package forkscope

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
)

// gitT runs git in dir with no user or system config, so a signing key or a
// hook on the machine cannot change what the test sees.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func writeT(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

// inherited is the parent's doc.md.
const inherited = "# Doc\n\nThe parser opens the file; it writes nothing.\n\nThe loader opens the file; it writes nothing.\n"

type forkFixture struct {
	parent, fork string
}

func newForkFixture(t *testing.T) forkFixture {
	t.Helper()
	work := t.TempDir()
	gitT(t, work, "init", "-q", "-b", "main")
	writeT(t, work, "doc.md", inherited)
	gitT(t, work, "add", "-A")
	gitT(t, work, "commit", "-q", "-m", "base")
	parent := filepath.Join(t.TempDir(), "parent.git")
	gitT(t, work, "clone", "-q", "--bare", work, parent)

	fork := filepath.Join(t.TempDir(), "fork")
	gitT(t, work, "clone", "-q", parent, fork)
	writeT(t, fork, "doc.md", "# Doc\n\nThe parser opens the file; it writes nothing.\n\nThe loader opens the tree; it writes nothing.\n\nThe fork adds this line; it writes nothing.\n")
	writeT(t, fork, "new.md", "# New\n\nThe fork adds this file; it writes nothing.\n")
	gitT(t, fork, "add", "-A")
	gitT(t, fork, "commit", "-q", "-m", "fork work")
	writeT(t, fork, "loose.md", "# Loose\n\nThe fork has not committed this; it writes nothing.\n")

	writeT(t, work, "later.md", "# Later\n\nThe parent adds this afterwards.\n")
	gitT(t, work, "add", "-A")
	gitT(t, work, "commit", "-q", "-m", "parent moves on")
	gitT(t, work, "push", "-q", parent, "main")
	return forkFixture{parent: parent, fork: fork}
}

// forkAPI serves GET /repos/o/fork the way GitHub answers for a fork of parent.
func forkAPI(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/fork" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func forkBody(cloneURL string) string {
	var info forkRepo
	info.Fork = true
	info.Parent = &struct {
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
	}{CloneURL: cloneURL, DefaultBranch: "main"}
	body, err := json.Marshal(info)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func envOf(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func forkEnv(srv *httptest.Server) func(string) string {
	return envOf(map[string]string{"GITHUB_REPOSITORY": "o/fork", "GITHUB_API_URL": srv.URL, "GITHUB_TOKEN": "tok"})
}

func forkLines(root string, getenv func(string) string, listURL string) (*Lines, error) {
	return Resolver{Getenv: getenv, ListURL: listURL}.Lines(root)
}

// prose names the lines of the fixture's documents that hold a sentence.
var prose = []int{3, 5, 7}

// held answers which of lines the fork wrote in the file name under root.
func held(own *Lines, root, name string, lines ...int) []int {
	out := []int{}
	for _, n := range lines {
		if own.Holds(filepath.Join(root, name), n, n) {
			out = append(out, n)
		}
	}
	return out
}

func scoped(t *testing.T, root string, getenv func(string) string, listURL string) *Lines {
	t.Helper()
	own, err := forkLines(root, getenv, listURL)
	require.NoError(t, err)
	require.NotNil(t, own)
	return own
}

func TestAForkOwnsOnlyTheLinesItWrote(t *testing.T) {
	fx := newForkFixture(t)
	own := scoped(t, fx.fork, forkEnv(forkAPI(t, forkBody(fx.parent))), noList(t))

	assert.Equal(t, []int{5, 7}, held(own, fx.fork, "doc.md", prose...), "line 3 is inherited, line 5 is edited, line 7 is added")
	assert.Equal(t, []int{3}, held(own, fx.fork, "new.md", 3), "a file the fork committed is the fork's")
	assert.Equal(t, []int{3}, held(own, fx.fork, "loose.md", 3), "an untracked file is the fork's")
	assert.True(t, own.Scope(filepath.Join(fx.fork, "later.md")).Empty(), "a file the fork never had is not the fork's")
}

// A fork's origin on GitHub names the repository, with no GITHUB_REPOSITORY.
// What the network said is kept in the git directory, so a second run asks nothing.
func TestTheOriginNamesTheForkAndTheAnswerIsKept(t *testing.T) {
	fx := newForkFixture(t)
	gitT(t, fx.fork, "remote", "set-url", "origin", "git@github.com:o/fork.git")
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		if r.URL.Path != "/repos/o/fork" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(forkBody(fx.parent)))
	}))
	t.Cleanup(srv.Close)
	env := envOf(map[string]string{"GITHUB_API_URL": srv.URL})

	own := scoped(t, fx.fork, env, srv.URL+"/fork-of.json")
	assert.Equal(t, []int{5, 7}, held(own, fx.fork, "doc.md", prose...))
	asked.Store(0)
	identities.Lock()
	clear(identities.byKey)
	identities.Unlock()

	own = scoped(t, fx.fork, env, srv.URL+"/fork-of.json")
	assert.Equal(t, []int{5, 7}, held(own, fx.fork, "doc.md", prose...))
	assert.Zero(t, asked.Load(), "the second run reads the record the first one kept")
}

func TestARepoOfURL(t *testing.T) {
	for raw, want := range map[string]string{
		"https://github.com/o/fork.git":       "o/fork",
		"https://github.com/o/fork":           "o/fork",
		"git@github.com:o/fork.git":           "o/fork",
		"ssh://git@github.com/o/fork.git":     "o/fork",
		"https://GitHub.com/o/fork/":          "o/fork",
		"https://gitlab.com/o/fork.git":       "",
		"/srv/git/parent.git":                 "",
		"https://github.com/o/fork/extra.git": "",
	} {
		assert.Equal(t, want, repoOfURL(raw, "github.com"), raw)
	}
}

// Inside an Actions run, GITHUB_REPOSITORY names the checkout. A root outside
// GITHUB_WORKSPACE is not that checkout, and nothing is asked about it.
func TestARootOutsideTheWorkspaceIsNotTheNamedRepository(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a request was made for a root outside GITHUB_WORKSPACE")
	}))
	t.Cleanup(srv.Close)
	env := envOf(map[string]string{"GITHUB_REPOSITORY": "o/fork", "GITHUB_API_URL": srv.URL, "GITHUB_WORKSPACE": t.TempDir()})
	own, err := forkLines(t.TempDir(), env, srv.URL+"/fork-of.json")
	require.NoError(t, err)
	assert.Nil(t, own)
}

// The event payload of an Actions run says whether the repository is a fork,
// so a repository that is not asks the API nothing.
func TestTheEventPayloadSparesTheAPI(t *testing.T) {
	fx := newForkFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fork-of.json" {
			http.NotFound(w, r)
			return
		}
		t.Error("the API was asked about a repository the event describes")
	}))
	t.Cleanup(srv.Close)
	event := filepath.Join(t.TempDir(), "event.json")
	writeT(t, filepath.Dir(event), "event.json", `{"repository":{"full_name":"O/Fork","fork":false}}`)
	env := envOf(map[string]string{"GITHUB_REPOSITORY": "o/fork", "GITHUB_API_URL": srv.URL, "GITHUB_EVENT_PATH": event})
	own, err := forkLines(fx.fork, env, srv.URL+"/fork-of.json")
	require.NoError(t, err)
	assert.Nil(t, own)
}

// A file's own lines come from a diff of its text against the base.
func TestBaseFileScopesAText(t *testing.T) {
	fx := newForkFixture(t)
	base, err := Resolver{Getenv: forkEnv(forkAPI(t, forkBody(fx.parent))), ListURL: noList(t)}.Base(fx.fork)
	require.NoError(t, err)
	require.NotNil(t, base)

	scope, err := base.File(filepath.Join(fx.fork, "doc.md"), inherited)
	require.NoError(t, err)
	assert.True(t, scope.Empty(), "the parent's text is none of the fork's")

	scope, err = base.File(filepath.Join(fx.fork, "doc.md"), inherited+"\nAdded.\n")
	require.NoError(t, err)
	assert.True(t, scope.Owns(6))
	assert.True(t, scope.Owns(7))
	assert.False(t, scope.Owns(3))

	scope, err = base.File(filepath.Join(fx.fork, "brand", "new.md"), "# New\n")
	require.NoError(t, err)
	assert.True(t, scope.All(), "a path the base does not hold is the fork's whole")
}

// Keep lands a change only where the fork wrote every line it replaces, and an
// insertion only beside a line the fork wrote.
func TestKeepPutsBackEveryChangeToAnInheritedLine(t *testing.T) {
	before := "a\nb\nc\nd\n"
	after := "A\nb\nC\nd\nE\n"
	assert.Equal(t, "a\nb\nC\nd\n", Keep(before, after, OfLines(3)))
	assert.Equal(t, "a\nb\nC\nd\nE\n", Keep(before, after, OfLines(3, 4)))
	assert.Equal(t, after, Keep(before, after, Whole()))
	assert.Equal(t, before, Keep(before, after, OfLines()))
	assert.Equal(t, "a\nb\nc", Keep("a\nb\nc", "a\nB\nc\n", OfLines(1)), "a line end is a change to its line")
}

func TestCarryFollowsTheLinesThroughARepair(t *testing.T) {
	carried := Carry("a\nb\nc\n", "a\nx\ny\nc\n", OfLines(2))
	assert.False(t, carried.Owns(1))
	assert.True(t, carried.Owns(2))
	assert.True(t, carried.Owns(3))
	assert.False(t, carried.Owns(4))
	assert.Equal(t, "b\n", Removed("a\nb\nc\n", "a\nx\ny\nc\n"))
}

func TestAShallowForkIsDeepenedBeforeTheMergeBase(t *testing.T) {
	fx := newForkFixture(t)
	gitT(t, fx.fork, "add", "-A")
	gitT(t, fx.fork, "commit", "-q", "-m", "loose")
	shallow := filepath.Join(t.TempDir(), "shallow")
	gitT(t, fx.fork, "clone", "-q", "--depth", "1", "file://"+fx.fork, shallow)
	require.Equal(t, "true", gitT(t, shallow, "rev-parse", "--is-shallow-repository"))

	own := scoped(t, shallow, forkEnv(forkAPI(t, forkBody(fx.parent))), noList(t))

	assert.Equal(t, []int{5, 7}, held(own, shallow, "doc.md", prose...))
	assert.Equal(t, []int{3}, held(own, shallow, "loose.md", 3))
	assert.Equal(t, "false", gitT(t, shallow, "rev-parse", "--is-shallow-repository"))
}

func TestAForkOwnsOnlyTheBinariesItWrote(t *testing.T) {
	const elf = "\x7fELF\x02\x01\x01\x00"
	work := t.TempDir()
	gitT(t, work, "init", "-q", "-b", "main")
	writeT(t, work, "inherited.bin", elf+"parent")
	writeT(t, work, "edited.bin", elf+"parent")
	gitT(t, work, "add", "-A")
	gitT(t, work, "commit", "-q", "-m", "base")
	parent := filepath.Join(t.TempDir(), "parent.git")
	gitT(t, work, "clone", "-q", "--bare", work, parent)

	fork := filepath.Join(t.TempDir(), "fork")
	gitT(t, work, "clone", "-q", parent, fork)
	writeT(t, fork, "edited.bin", elf+"fork")
	writeT(t, fork, "added.bin", elf+"fork")
	gitT(t, fork, "add", "-A")
	gitT(t, fork, "commit", "-q", "-m", "fork work")

	own := scoped(t, fork, forkEnv(forkAPI(t, forkBody(parent))), noList(t))
	assert.True(t, own.Whole(filepath.Join(fork, "added.bin")), "a binary the fork added is the fork's")
	assert.True(t, own.Whole(filepath.Join(fork, "edited.bin")), "a binary the fork changed is the fork's")
	assert.False(t, own.Holds(filepath.Join(fork, "inherited.bin"), 0, 0), "a binary it inherited unchanged is not")
}

func TestBinaryDiffPath(t *testing.T) {
	for pair, want := range map[string]string{
		"/dev/null and b/new.bin":                 "new.bin",
		"a/x.bin and b/x.bin":                     "x.bin",
		"a/this and that and b/this and that":     "this and that",
		`"a/t\303\251.bin" and "b/t\303\251.bin"`: "té.bin",
		"a/gone.bin and /dev/null":                "",
	} {
		got, err := binaryDiffPath(pair)
		require.NoError(t, err, pair)
		assert.Equal(t, want, got, pair)
	}
	_, err := binaryDiffPath("a/x and b/yy")
	assert.Error(t, err, "sides that name different paths are refused")
}

func TestARepositoryThatIsNoForkHasNoScope(t *testing.T) {
	fx := newForkFixture(t)
	own, err := forkLines(fx.fork, forkEnv(forkAPI(t, `{"fork":false}`)), noList(t))
	require.NoError(t, err)
	assert.Nil(t, own)
}

// tagParent tags the parent. The fork carries the parent's base commit, tagged
// annotated as v1. The parent's later commit, tagged v2, is one the fork never merged.
func tagParent(t *testing.T, fx forkFixture) {
	t.Helper()
	base := gitT(t, fx.fork, "rev-parse", "HEAD~1")
	gitT(t, fx.parent, "tag", "-a", "-m", "v1", "v1", base)
	gitT(t, fx.parent, "tag", "v2", "main")
}

// listAt serves list at its URL, with status, and answers that URL.
func listAt(t *testing.T, status int, list string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/fork-of.json", r.URL.Path)
		assert.Empty(t, r.Header.Get("Authorization"), "the list is public, so no token goes to it")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(list))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/fork-of.json"
}

// forkList answers a fork list with one entry, name to upstream.
func forkList(name, upstream string) string {
	return fmt.Sprintf("{%q: %q}\n", name, upstream)
}

// noList answers the URL of a fork list that does not exist.
func noList(t *testing.T) string {
	t.Helper()
	return listAt(t, http.StatusNotFound, "not found")
}

// listedEnv names the repository o/fork. Any API call fails the test, because a listed fork needs none.
func listedEnv(t *testing.T) func(string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the API was asked about a fork the list names")
	}))
	t.Cleanup(srv.Close)
	return forkEnv(srv)
}

func TestAListedForkReportsOnlyTheLinesItWrote(t *testing.T) {
	fx := newForkFixture(t)
	tagParent(t, fx)
	list := fmt.Sprintf("// Forks GitHub does not record.\n{\n\t\"x/other\": \"https://example.invalid/other\", /* a fork of another */\n\t\"O/Fork\": %q,\n}\n", fx.parent)
	own := scoped(t, fx.fork, listedEnv(t), listAt(t, http.StatusOK, list))

	assert.Equal(t, []int{5, 7}, held(own, fx.fork, "doc.md", prose...), "line 3 is upstream's, line 5 is edited, line 7 is added")
	assert.Equal(t, []int{3}, held(own, fx.fork, "new.md", 3))
	assert.Equal(t, []int{3}, held(own, fx.fork, "loose.md", 3))
}

// A repository cannot declare itself a fork: a file in its own tree is no list.
func TestAForkFileInTheRepositoryIsIgnored(t *testing.T) {
	fx := newForkFixture(t)
	tagParent(t, fx)
	require.NoError(t, os.MkdirAll(filepath.Join(fx.fork, ".github"), 0o755))
	writeT(t, fx.fork, filepath.Join(".github", "fork-of.json"), forkList("o/fork", fx.parent))
	own, err := forkLines(fx.fork, forkEnv(forkAPI(t, `{"fork":false}`)), noList(t))
	require.NoError(t, err)
	assert.Nil(t, own, "only the org's list names a fork")
}

// The list names a repository with its owner, so a same-named repository of another owner is no fork.
func TestAListEntryOfAnotherOwnerIsNoFork(t *testing.T) {
	fx := newForkFixture(t)
	tagParent(t, fx)
	own, err := forkLines(fx.fork, forkEnv(forkAPI(t, `{"fork":false}`)), listAt(t, http.StatusOK, forkList("other/fork", fx.parent)))
	require.NoError(t, err)
	assert.Nil(t, own)
}

func TestAShallowListedForkIsDeepenedFirst(t *testing.T) {
	fx := newForkFixture(t)
	tagParent(t, fx)
	gitT(t, fx.fork, "add", "-A")
	gitT(t, fx.fork, "commit", "-q", "-m", "loose")
	shallow := filepath.Join(t.TempDir(), "shallow")
	gitT(t, fx.fork, "clone", "-q", "--depth", "1", "file://"+fx.fork, shallow)

	own := scoped(t, shallow, listedEnv(t), listAt(t, http.StatusOK, forkList("o/fork", fx.parent)))

	assert.Equal(t, []int{5, 7}, held(own, shallow, "doc.md", prose...))
	assert.Equal(t, "false", gitT(t, shallow, "rev-parse", "--is-shallow-repository"))
}

func TestAListedForkWithNoUsableTagFailsLoudly(t *testing.T) {
	untagged := t.TempDir()
	gitT(t, untagged, "init", "-q", "-b", "main")
	writeT(t, untagged, "other.md", "# Other\n")
	gitT(t, untagged, "add", "-A")
	gitT(t, untagged, "commit", "-q", "-m", "other")
	unrelated := filepath.Join(t.TempDir(), "unrelated.git")
	gitT(t, untagged, "clone", "-q", "--bare", untagged, unrelated)
	gitT(t, unrelated, "tag", "v1", "main")

	cases := map[string]struct {
		status int
		list   string
		want   string
	}{
		"an entry without a URL":   {http.StatusOK, forkList("o/fork", ""), "is not a fork list"},
		"a name with no owner":     {http.StatusOK, forkList("fork", untagged), "is not a fork list"},
		"a URL that is no string":  {http.StatusOK, `{"o/fork": 1}`, "is not a fork list"},
		"a list that is no object": {http.StatusOK, `["o/fork"]`, "is not a fork list"},
		"a null list":              {http.StatusOK, "null", "is not a fork list"},
		"a list that is no JSON":   {http.StatusOK, "o/fork " + untagged + "\n", "is not a fork list"},
		"a list error":             {http.StatusInternalServerError, "boom", "500"},
		"missing upstream":         {http.StatusOK, forkList("o/fork", filepath.Join(t.TempDir(), "gone.git")), "list the tags of"},
		"no tags":                  {http.StatusOK, forkList("o/fork", untagged), "has no tags"},
		"unrelated tags":           {http.StatusOK, forkList("o/fork", unrelated), "HEAD contains none of the tags"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newForkFixture(t)
			own, err := forkLines(fx.fork, listedEnv(t), listAt(t, c.status, c.list))
			require.Error(t, err)
			assert.Nil(t, own)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

// An org with no list leaves the decision to the API.
func TestAnOrgWithNoListAsksTheAPI(t *testing.T) {
	fx := newForkFixture(t)
	own, err := forkLines(fx.fork, forkEnv(forkAPI(t, `{"fork":false}`)), noList(t))
	require.NoError(t, err)
	assert.Nil(t, own)
}

func TestOutsideActionsNoRequestIsMade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("a request was made with no GITHUB_REPOSITORY set")
	}))
	t.Cleanup(srv.Close)
	own, err := forkLines(t.TempDir(), envOf(map[string]string{"GITHUB_API_URL": srv.URL}), srv.URL+"/fork-of.json")
	require.NoError(t, err)
	assert.Nil(t, own)
}

func TestAForkWithNoReachableBaseFailsLoudly(t *testing.T) {
	fx := newForkFixture(t)
	unrelated := t.TempDir()
	gitT(t, unrelated, "init", "-q", "-b", "main")
	writeT(t, unrelated, "other.md", "# Other\n")
	gitT(t, unrelated, "add", "-A")
	gitT(t, unrelated, "commit", "-q", "-m", "other")

	cases := map[string]struct {
		body string
		want string
	}{
		"missing parent":   {forkBody(filepath.Join(t.TempDir(), "gone.git")), "fetch the parent's main"},
		"unrelated parent": {forkBody(unrelated), "no merge base"},
		"no parent named":  {`{"fork":true}`, "names no parent"},
		"bad JSON":         {`{"fork":`, "GET "},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			own, err := forkLines(fx.fork, forkEnv(forkAPI(t, c.body)), noList(t))
			require.Error(t, err)
			assert.Nil(t, own)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

func TestAnAPIErrorFailsLoudly(t *testing.T) {
	srv := forkAPI(t, forkBody("unused"))
	own, err := forkLines(t.TempDir(), envOf(map[string]string{"GITHUB_REPOSITORY": "o/fork", "GITHUB_API_URL": srv.URL}), noList(t))
	require.Error(t, err)
	assert.Nil(t, own)
	assert.Contains(t, err.Error(), "401")

	own, err = forkLines(t.TempDir(), forkEnv(srv), "http://127.0.0.1:1/fork-of.json")
	require.Error(t, err)
	assert.Nil(t, own)
	assert.Contains(t, err.Error(), "fork scope: GET http://127.0.0.1:1/fork-of.json", "the fork list is asked for first")
}

func TestAForkOutsideAWorkTreeFailsLoudly(t *testing.T) {
	_, err := forkLines(t.TempDir(), forkEnv(forkAPI(t, forkBody("unused"))), noList(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rev-parse --show-toplevel")
}

// A content line that reads like a header is counted as hunk body, and a
// quoted path and a deleted file are read.
func TestTheDiffReaderCountsHunkBodies(t *testing.T) {
	diff := strings.Join([]string{
		`diff --git a/x.md b/x.md`,
		`--- a/x.md`,
		`+++ b/x.md`,
		`@@ -2 +2,2 @@`,
		`-old`,
		`+++ b/fake.md`,
		`+@@ -1 +1,9 @@`,
		`\ No newline at end of file`,
		`diff --git "a/sp ace\t.md" "b/sp ace\t.md"`,
		`--- /dev/null`,
		`+++ "b/sp ace\t.md"`,
		`@@ -0,0 +1 @@`,
		`+new`,
		`diff --git a/gone.md b/gone.md`,
		`--- a/gone.md`,
		`+++ /dev/null`,
		`@@ -1 +0,0 @@`,
		`-gone`,
		``,
	}, "\n")
	own := &Lines{top: "/", whole: set.New[string](), lines: map[string]set.Set[int]{}}
	require.NoError(t, own.readDiff(diff))

	assert.True(t, own.Holds("/x.md", 2, 0))
	assert.True(t, own.Holds("/x.md", 1, 3))
	assert.False(t, own.Holds("/x.md", 1, 1))
	assert.True(t, own.Holds("/x.md", 0, 0), "a file finding holds when any line changed")
	assert.False(t, own.Holds("/fake.md", 1, 1))
	assert.True(t, own.Holds("/sp ace\t.md", 40, 40))
	assert.False(t, own.Holds("/gone.md", 1, 1))
	assert.False(t, own.Holds("/untouched.md", 0, 0))
}

func TestTheDiffReaderRefusesWhatItCannotRead(t *testing.T) {
	for name, diff := range map[string]string{
		"bad header":   "@@ nonsense\n",
		"bad range":    "@@ -1 +x @@\n",
		"bad quote":    "+++ \"b/x\n",
		"cut off hunk": "+++ b/x\n@@ -1 +1 @@\n-a\n",
	} {
		t.Run(name, func(t *testing.T) {
			own := &Lines{top: "/", whole: set.New[string](), lines: map[string]set.Set[int]{}}
			assert.Error(t, own.readDiff(diff))
		})
	}
}
