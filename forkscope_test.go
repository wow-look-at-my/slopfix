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

func scopedCheck(t *testing.T, root string, getenv func(string) string) TreeRepair {
	t.Helper()
	own, err := ForkLines(root, getenv)
	require.NoError(t, err)
	require.NotNil(t, own)
	return CheckTreeWith(root, Request{}).Within(own, root)
}

func TestAForkReportsOnlyTheLinesItWrote(t *testing.T) {
	fx := newForkFixture(t)
	out := scopedCheck(t, fx.fork, forkEnv(forkAPI(t, forkBody(fx.parent))))

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

	out := scopedCheck(t, shallow, forkEnv(forkAPI(t, forkBody(fx.parent))))

	assert.Equal(t, []int{5, 7}, semicolons(out, "doc.md"))
	assert.Equal(t, []int{3}, semicolons(out, "loose.md"))
	assert.Equal(t, "false", gitT(t, shallow, "rev-parse", "--is-shallow-repository"))
}

func TestARepositoryThatIsNoForkKeepsEveryFinding(t *testing.T) {
	fx := newForkFixture(t)
	own, err := ForkLines(fx.fork, forkEnv(forkAPI(t, `{"fork":false}`)))
	require.NoError(t, err)
	assert.Nil(t, own)
	out := CheckTreeWith(fx.fork, Request{}).Within(own, fx.fork)
	assert.Equal(t, []int{3, 5, 7}, semicolons(out, "doc.md"))
}

func TestOutsideActionsNoRequestIsMade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the API was asked with no GITHUB_REPOSITORY set")
	}))
	t.Cleanup(srv.Close)
	own, err := ForkLines(t.TempDir(), envOf(map[string]string{"GITHUB_API_URL": srv.URL}))
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
			own, err := ForkLines(fx.fork, forkEnv(forkAPI(t, c.body)))
			require.Error(t, err)
			assert.Nil(t, own)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

func TestAnAPIErrorFailsLoudly(t *testing.T) {
	srv := forkAPI(t, forkBody("unused"))
	own, err := ForkLines(t.TempDir(), envOf(map[string]string{"GITHUB_REPOSITORY": "o/fork", "GITHUB_API_URL": srv.URL}))
	require.Error(t, err)
	assert.Nil(t, own)
	assert.Contains(t, err.Error(), "401")

	own, err = ForkLines(t.TempDir(), envOf(map[string]string{"GITHUB_REPOSITORY": "o/fork", "GITHUB_API_URL": "http://127.0.0.1:1"}))
	require.Error(t, err)
	assert.Nil(t, own)
	assert.Contains(t, err.Error(), "fork scope: GET http://127.0.0.1:1/repos/o/fork")
}

func TestAForkOutsideAWorkTreeFailsLoudly(t *testing.T) {
	_, err := ForkLines(t.TempDir(), forkEnv(forkAPI(t, forkBody("unused"))))
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
