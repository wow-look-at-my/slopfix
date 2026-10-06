package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/forkscope"
)

// A test that walks a directory through runCheck reads os.Getenv.
func TestMain(m *testing.M) {
	if err := os.Unsetenv("GITHUB_REPOSITORY"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// noForks finds no fork: no origin is on its server, and it names no repository.
var noForks = forkscope.Resolver{Getenv: func(key string) string {
	if key == "GITHUB_SERVER_URL" {
		return "https://ghe.invalid"
	}
	return ""
}}

// noForkLines answers every text as in no fork.
func noForkLines(string) (*forkscope.Scope, error) { return nil, nil }

// A fork whose base cannot be read fails the check with the reason, and reports nothing.
func TestATreeCheckInAForkWithNoBaseFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	env := map[string]string{"GITHUB_REPOSITORY": "o/fork", "GITHUB_API_URL": srv.URL}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	forks := forkscope.Resolver{Getenv: func(k string) string { return env[k] }, ListURL: srv.URL + "/fork-of.json"}
	failed, err := treeFindings(cmd, t.TempDir(), slopfix.Request{}, false, forks, nil)
	require.Error(t, err)
	assert.False(t, failed)
	assert.Contains(t, err.Error(), "500")
	assert.Empty(t, out.String())
}

// upstreamDoc is a file of the parent that the fork never touches.
const upstreamDoc = "# Up\n\nIt doesn't hold the lock.\n"

// parentDoc is the parent's doc.md. The fork rewrites its last line.
const parentDoc = "# Doc\n\nIt doesn't hold the lock.\n\nIt doesn't free the lock.\n"

// forkDoc is doc.md as the fork has it.
const forkDoc = "# Doc\n\nIt doesn't hold the lock.\n\nIt doesn't free the page.\n"

// forkRepo is a fork, its origin on GitHub, and the resolver that reaches its parent.
type forkRepo struct {
	dir   string
	forks forkscope.Resolver
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

func put(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

// newForkRepo builds a parent with upstream.md and doc.md. It also builds a
// fork that rewrites a line of doc.md and adds mine.md. The fork's origin names
// o/fork on GitHub. The server api builds, from the parent's clone URL,
// answers both the API and the fork list.
func newForkRepo(t *testing.T, api func(parent string) http.HandlerFunc) forkRepo {
	t.Helper()
	work := t.TempDir()
	gitRun(t, work, "init", "-q", "-b", "main")
	put(t, work, "upstream.md", upstreamDoc)
	put(t, work, "doc.md", parentDoc)
	gitRun(t, work, "add", "-A")
	gitRun(t, work, "commit", "-q", "-m", "base")
	parent := filepath.Join(t.TempDir(), "parent.git")
	gitRun(t, work, "clone", "-q", "--bare", work, parent)

	fork := filepath.Join(t.TempDir(), "fork")
	gitRun(t, work, "clone", "-q", parent, fork)
	put(t, fork, "doc.md", forkDoc)
	put(t, fork, "mine.md", "# Mine\n\nIt doesn't wait.\n")
	gitRun(t, fork, "add", "-A")
	gitRun(t, fork, "commit", "-q", "-m", "fork work")
	gitRun(t, fork, "remote", "set-url", "origin", "https://github.com/o/fork.git")

	srv := httptest.NewServer(api(parent))
	t.Cleanup(srv.Close)
	env := map[string]string{"GITHUB_API_URL": srv.URL}
	return forkRepo{dir: fork, forks: forkscope.Resolver{Getenv: func(k string) string { return env[k] }, ListURL: srv.URL + "/fork-of.json"}}
}

// answers serves the fork list as missing, and GET /repos/o/fork as repo.
func answers(repo func(parent string) any) func(parent string) http.HandlerFunc {
	return func(parent string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/o/fork" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(repo(parent))
		}
	}
}

// aFork answers that o/fork is a fork of parent's main.
var aFork = answers(func(parent string) any {
	return map[string]any{"fork": true, "parent": map[string]any{"clone_url": parent, "default_branch": "main"}}
})

// noFork answers that o/fork is no fork.
var noFork = answers(func(string) any { return map[string]any{"fork": false} })

// broken answers every request with a server error.
func broken(string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", http.StatusInternalServerError) }
}

func readT(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func nthLine(text string, n int) string {
	return strings.Split(text, "\n")[n-1]
}

func quietCmd() (*cobra.Command, *bytes.Buffer) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	return cmd, &out
}

// poisonGit puts a git on PATH that records every spawn, then exits nonzero.
// It answers the marker path, which the check must leave absent.
func poisonGit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "spawned")
	script := "#!/bin/sh\nprintf spawned >> \"$SLOPFIX_GIT_MARKER\"\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755))
	t.Setenv("SLOPFIX_GIT_MARKER", marker)
	t.Setenv("PATH", dir)
	return marker
}

// `slopfix check .` in a fork reads the repository itself, so the whole check
// starts no git process.
func TestATreeCheckInAForkStartsNoGitProcess(t *testing.T) {
	fx := newForkRepo(t, aFork)
	marker := poisonGit(t)

	cmd, out := quietCmd()
	failed, err := treeFindings(cmd, fx.dir, slopfix.Request{}, false, fx.forks, nil)
	require.NoError(t, err)
	assert.False(t, failed, out.String())
	assert.NoFileExists(t, marker, "a git process was spawned on the check path")
}

// `slopfix fix .` in a fork leaves every file the fork never touched as it
// was. In a file the fork touched, it repairs only the lines the fork wrote.
func TestFixOfAForkTreeKeepsToTheForksLines(t *testing.T) {
	fx := newForkRepo(t, aFork)
	cmd, out := quietCmd()
	_, err := treeFindings(cmd, fx.dir, slopfix.Request{}, true, fx.forks, nil)
	require.NoError(t, err)

	assert.Equal(t, upstreamDoc, readT(t, filepath.Join(fx.dir, "upstream.md")), "a file the fork never touched stays byte for byte")
	doc := readT(t, filepath.Join(fx.dir, "doc.md"))
	assert.Equal(t, "It doesn't hold the lock.", nthLine(doc, 3), "an inherited line stays")
	assert.NotContains(t, nthLine(doc, 5), "doesn't", "the line the fork wrote is repaired")
	assert.NotContains(t, readT(t, filepath.Join(fx.dir, "mine.md")), "doesn't", "a file the fork added is repaired whole")
	assert.NotContains(t, out.String(), "upstream.md")
}

// In a fork, the files the budget move creates are the fork's, so the same `fix` repairs them and the check after it passes.
func TestFixOfAForkRepairsTheDocsItsBudgetMoveCreates(t *testing.T) {
	fx := newForkRepo(t, aFork)
	var agents strings.Builder
	agents.WriteString("# Agents\n")
	paragraph := strings.Repeat("It doesn't wait for the lock. ", 20)
	for section := range 12 {
		fmt.Fprintf(&agents, "\n## Section %c\n", 'a'+section)
		for range 6 {
			agents.WriteString("\n" + strings.TrimSpace(paragraph) + "\n")
		}
	}
	put(t, fx.dir, "AGENTS.md", agents.String())
	gitRun(t, fx.dir, "add", "-A")
	gitRun(t, fx.dir, "commit", "-q", "-m", "a large AGENTS.md")

	cmd, out := quietCmd()
	_, err := treeFindings(cmd, fx.dir, slopfix.Request{}, true, fx.forks, nil)
	require.NoError(t, err)
	docs, err := filepath.Glob(filepath.Join(fx.dir, "docs", "*.md"))
	require.NoError(t, err)
	require.NotEmpty(t, docs, "the move wrote no docs:\n%s", out.String())
	for _, doc := range docs {
		assert.NotContains(t, readT(t, doc), "doesn't", "%s is left as the move wrote it", doc)
	}

	cmd, out = quietCmd()
	failed, err := treeFindings(cmd, fx.dir, slopfix.Request{}, false, fx.forks, nil)
	require.NoError(t, err)
	assert.False(t, failed, "the check after the fix still fails:\n%s", out.String())
}

// A repository that is no fork is repaired whole, as before.
func TestFixOfATreeThatIsNoForkRepairsEveryFile(t *testing.T) {
	fx := newForkRepo(t, noFork)
	cmd, _ := quietCmd()
	_, err := treeFindings(cmd, fx.dir, slopfix.Request{}, true, fx.forks, nil)
	require.NoError(t, err)

	assert.NotContains(t, readT(t, filepath.Join(fx.dir, "upstream.md")), "doesn't")
	assert.NotContains(t, readT(t, filepath.Join(fx.dir, "doc.md")), "doesn't")
}

// A fork whose base cannot be read writes nothing and fails with the reason.
func TestFixOfAForkWithNoBaseWritesNothing(t *testing.T) {
	fx := newForkRepo(t, broken)
	cmd, _ := quietCmd()
	_, err := treeFindings(cmd, fx.dir, slopfix.Request{}, true, fx.forks, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Equal(t, upstreamDoc, readT(t, filepath.Join(fx.dir, "upstream.md")))
	assert.Equal(t, forkDoc, readT(t, filepath.Join(fx.dir, "doc.md")))

	_, err = repairOf(fx.forks, nil, filepath.Join(fx.dir, "doc.md"), slopfix.Request{}, true)
	require.Error(t, err)
	assert.Equal(t, forkDoc, readT(t, filepath.Join(fx.dir, "doc.md")))
}

// `slopfix fix FILE` on a file the fork never touched neither reads nor
// writes it. On a file the fork touched it repairs only the fork's lines.
func TestFixOfANamedFileInAForkKeepsToTheForksLines(t *testing.T) {
	fx := newForkRepo(t, aFork)

	repair, err := repairOf(fx.forks, nil, filepath.Join(fx.dir, "upstream.md"), slopfix.Request{}, true)
	require.NoError(t, err)
	assert.False(t, repair.Changed)
	assert.Empty(t, repair.Findings)
	assert.Equal(t, upstreamDoc, readT(t, filepath.Join(fx.dir, "upstream.md")))

	repair, err = repairOf(fx.forks, nil, filepath.Join(fx.dir, "upstream.md"), slopfix.Request{}, false)
	require.NoError(t, err)
	assert.Empty(t, repair.Findings)
	assert.Empty(t, repair.Kept, "a check of a file the fork never touched finds nothing")

	_, err = repairOf(fx.forks, nil, filepath.Join(fx.dir, "doc.md"), slopfix.Request{}, true)
	require.NoError(t, err)
	doc := readT(t, filepath.Join(fx.dir, "doc.md"))
	assert.Equal(t, "It doesn't hold the lock.", nthLine(doc, 3))
	assert.NotContains(t, nthLine(doc, 5), "doesn't")
}

// forkAsk is ask through the fork's own resolver.
func forkAsk(t *testing.T, fx forkRepo, payload map[string]any) answer {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	body := judge(data, nil, nil, fx.forks)
	if body == "" {
		return answer{}
	}
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &raw))
	out, _ := raw["hookSpecificOutput"].(map[string]any)
	return answer{raw: raw, out: out, body: body}
}

// The write hook in a fork repairs only what the write makes the fork's.
func TestTheWriteHookInAForkKeepsToTheForksLines(t *testing.T) {
	fx := newForkRepo(t, aFork)
	path := filepath.Join(fx.dir, "upstream.md")

	got := forkAsk(t, fx, write(path, upstreamDoc+"\nIt doesn't stop.\n"))
	require.NotNil(t, got.out, got.body)
	updated, _ := got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated, got.body)
	content, _ := updated["content"].(string)
	assert.True(t, strings.HasPrefix(content, upstreamDoc), "the inherited lines stay as they were: %q", content)
	assert.NotContains(t, strings.TrimPrefix(content, upstreamDoc), "doesn't", "the line the write adds is repaired")

	got = forkAsk(t, fx, map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Edit",
		"tool_input":      map[string]any{"file_path": path, "old_string": "# Up\n", "new_string": "# Up\n\nIt doesn't wait.\n"},
	})
	require.NotNil(t, got.out, got.body)
	assert.NotContains(t, got.body, "permissionDecision", "an edit beside an inherited line that fix would repair is not refused")
	updated, _ = got.out["updatedInput"].(map[string]any)
	require.NotNil(t, updated, got.body)
	assert.NotContains(t, updated["new_string"], "doesn't")
	assert.Equal(t, upstreamDoc, readT(t, path), "the hook writes nothing itself")
}

// The write hook in a fork whose base cannot be read refuses the write.
func TestTheWriteHookInAForkWithNoBaseRefuses(t *testing.T) {
	fx := newForkRepo(t, broken)
	got := forkAsk(t, fx, write(filepath.Join(fx.dir, "upstream.md"), upstreamDoc))
	require.NotNil(t, got.out, got.body)
	assert.Equal(t, "deny", got.out["permissionDecision"])
	assert.Contains(t, got.out["permissionDecisionReason"], "500")
}
