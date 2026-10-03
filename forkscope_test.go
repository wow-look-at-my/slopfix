package slopfix

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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

// semicolons answers the lines of name that hold a ste/semicolon finding.
func semicolons(out TreeRepair, name string) []int {
	var lines []int
	for _, f := range out.Findings {
		if filepath.Base(f.Path) == name && f.ID == "ste/semicolon" {
			lines = append(lines, f.Line)
		}
	}
	slices.Sort(lines)
	return lines
}

func scopedCheck(t *testing.T, root string, getenv func(string) string, listURL string) TreeRepair {
	t.Helper()
	own, err := forkLines(root, getenv, listURL)
	require.NoError(t, err)
	require.NotNil(t, own)
	return CheckTreeWith(root, Request{}).Within(own, root)
}

func TestAForkReportsOnlyTheLinesItWrote(t *testing.T) {
	fx := newForkFixture(t)
	out := scopedCheck(t, fx.fork, forkEnv(forkAPI(t, forkBody(fx.parent))), noList(t))

	assert.Equal(t, []int{5, 7}, semicolons(out, "doc.md"), "line 3 is inherited, line 5 is edited, line 7 is added")
	assert.Equal(t, []int{3}, semicolons(out, "new.md"), "a file the fork committed is the fork's")
	assert.Equal(t, []int{3}, semicolons(out, "loose.md"), "an untracked file is the fork's")
}

func TestAShallowForkIsDeepenedBeforeTheMergeBase(t *testing.T) {
	fx := newForkFixture(t)
	gitT(t, fx.fork, "add", "-A")
	gitT(t, fx.fork, "commit", "-q", "-m", "loose")
	shallow := filepath.Join(t.TempDir(), "shallow")
	gitT(t, fx.fork, "clone", "-q", "--depth", "1", "file://"+fx.fork, shallow)
	require.Equal(t, "true", gitT(t, shallow, "rev-parse", "--is-shallow-repository"))

	out := scopedCheck(t, shallow, forkEnv(forkAPI(t, forkBody(fx.parent))), noList(t))

	assert.Equal(t, []int{5, 7}, semicolons(out, "doc.md"))
	assert.Equal(t, []int{3}, semicolons(out, "loose.md"))
	assert.Equal(t, "false", gitT(t, shallow, "rev-parse", "--is-shallow-repository"))
}

func TestARepositoryThatIsNoForkKeepsEveryFinding(t *testing.T) {
	fx := newForkFixture(t)
	own, err := forkLines(fx.fork, forkEnv(forkAPI(t, `{"fork":false}`)), noList(t))
	require.NoError(t, err)
	assert.Nil(t, own)
	out := CheckTreeWith(fx.fork, Request{}).Within(own, fx.fork)
	assert.Equal(t, []int{3, 5, 7}, semicolons(out, "doc.md"))
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
		assert.Equal(t, "/fork-of", r.URL.Path)
		assert.Empty(t, r.Header.Get("Authorization"), "the list is public, so no token goes to it")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(list))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/fork-of"
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
	list := "# Forks GitHub does not record.\n\nx/other https://example.invalid/other\nO/Fork " + fx.parent + "\n"
	out := scopedCheck(t, fx.fork, listedEnv(t), listAt(t, http.StatusOK, list))

	assert.Equal(t, []int{5, 7}, semicolons(out, "doc.md"), "line 3 is upstream's, line 5 is edited, line 7 is added")
	assert.Equal(t, []int{3}, semicolons(out, "new.md"))
	assert.Equal(t, []int{3}, semicolons(out, "loose.md"))
}

// A repository cannot declare itself a fork: a file in its own tree is no list.
func TestAForkFileInTheRepositoryIsIgnored(t *testing.T) {
	fx := newForkFixture(t)
	tagParent(t, fx)
	require.NoError(t, os.MkdirAll(filepath.Join(fx.fork, ".github"), 0o755))
	writeT(t, fx.fork, filepath.Join(".github", "fork-of"), "o/fork "+fx.parent+"\n")
	own, err := forkLines(fx.fork, forkEnv(forkAPI(t, `{"fork":false}`)), noList(t))
	require.NoError(t, err)
	assert.Nil(t, own, "only the org's list names a fork")
}

// The list names a repository with its owner, so a same-named repository of another owner is no fork.
func TestAListEntryOfAnotherOwnerIsNoFork(t *testing.T) {
	fx := newForkFixture(t)
	tagParent(t, fx)
	own, err := forkLines(fx.fork, forkEnv(forkAPI(t, `{"fork":false}`)), listAt(t, http.StatusOK, "other/fork "+fx.parent+"\n"))
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

	out := scopedCheck(t, shallow, listedEnv(t), listAt(t, http.StatusOK, "o/fork "+fx.parent+"\n"))

	assert.Equal(t, []int{5, 7}, semicolons(out, "doc.md"))
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
		"a line without a URL": {http.StatusOK, "o/fork\n", "line 1 is not an OWNER/NAME and an upstream URL"},
		"a name with no owner": {http.StatusOK, "fork " + untagged + "\n", "line 1 is not an OWNER/NAME and an upstream URL"},
		"a list error":         {http.StatusInternalServerError, "boom", "500"},
		"missing upstream":     {http.StatusOK, "o/fork " + filepath.Join(t.TempDir(), "gone.git") + "\n", "list the tags of"},
		"no tags":              {http.StatusOK, "o/fork " + untagged + "\n", "has no tags"},
		"unrelated tags":       {http.StatusOK, "o/fork " + unrelated + "\n", "HEAD contains none of the tags"},
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
	own, err := forkLines(t.TempDir(), envOf(map[string]string{"GITHUB_API_URL": srv.URL}), srv.URL+"/fork-of")
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

	own, err = forkLines(t.TempDir(), forkEnv(srv), "http://127.0.0.1:1/fork-of")
	require.Error(t, err)
	assert.Nil(t, own)
	assert.Contains(t, err.Error(), "fork scope: GET http://127.0.0.1:1/fork-of", "the fork list is asked for first")
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
	own := &OwnLines{top: "/", whole: set.New[string](), lines: map[string]set.Set[int]{}}
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
			own := &OwnLines{top: "/", whole: set.New[string](), lines: map[string]set.Set[int]{}}
			assert.Error(t, own.readDiff(diff))
		})
	}
}
