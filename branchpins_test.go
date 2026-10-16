package slopfix_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// refsAPI serves the endpoints GitHubRefs reads for the repository o/r, and
// counts the requests it answers.
func refsAPI(t *testing.T, calls *int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		switch r.URL.EscapedPath() {
		case "/repos/o/r":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"default_branch": "master"}))
		case "/repos/o/r/git/ref/heads/feature", "/repos/o/r/git/ref/heads/master":
			w.WriteHeader(http.StatusOK)
		case "/repos/o/r/git/ref/tags/v1", "/repos/o/r/git/ref/tags/act%23latest":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGitHubRefsReadsTheAPI(t *testing.T) {
	calls := 0
	srv := refsAPI(t, &calls)
	env := map[string]string{"GITHUB_API_URL": srv.URL, "GITHUB_TOKEN": "tok"}
	refs := slopfix.NewGitHubRefs(func(k string) string { return env[k] })

	def, err := refs.DefaultBranch("o/r")
	require.NoError(t, err)
	assert.Equal(t, "master", def)
	for ref, want := range map[string]workflow.RefKind{
		"feature":    workflow.RefBranch,
		"master":     workflow.RefBranch,
		"v1":         workflow.RefTag,
		"act#latest": workflow.RefTag,
		"gone":       workflow.RefMissing,
	} {
		kind, err := refs.Kind("o/r", ref)
		require.NoError(t, err, ref)
		assert.Equal(t, want, kind, ref)
	}
	before := calls
	_, err = refs.Kind("o/r", "feature")
	require.NoError(t, err)
	assert.Equal(t, before, calls, "an answer is kept")
}

func TestGitHubRefsFailsOnARepositoryItCannotRead(t *testing.T) {
	calls := 0
	srv := refsAPI(t, &calls)
	env := map[string]string{"GITHUB_API_URL": srv.URL, "GITHUB_TOKEN": "tok"}
	refs := slopfix.NewGitHubRefs(func(k string) string { return env[k] })
	_, err := refs.Kind("o/private", "v1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestBranchPinsTreeReportsAWorkflowFinding(t *testing.T) {
	calls := 0
	srv := refsAPI(t, &calls)
	env := map[string]string{"GITHUB_API_URL": srv.URL, "GITHUB_TOKEN": "tok"}
	refs := slopfix.NewGitHubRefs(func(k string) string { return env[k] })
	root := t.TempDir()
	dir := filepath.Join(root, ".github", "workflows")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	ci := filepath.Join(dir, "ci.yml")
	require.NoError(t, os.WriteFile(ci, []byte("on: {push: {branches: ['**']}}\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: o/r@master\n      - uses: o/r@feature\n"), 0o644))

	pins := func(req slopfix.Request) []slopfix.TreeFinding {
		var found []slopfix.TreeFinding
		for _, f := range slopfix.CheckTreeWith(root, req).Findings {
			if f.ID == workflow.IDBranchPin {
				found = append(found, f)
			}
		}
		return found
	}
	found := pins(slopfix.Request{Refs: refs})
	require.Len(t, found, 1)
	assert.Equal(t, ci, found[0].Path)
	assert.Equal(t, 7, found[0].Line)

	assert.Empty(t, pins(slopfix.Request{Refs: refs, Rules: []slopfix.Rule{slopfix.RuleRepo}}), "a selection without the workflow rules asks nothing")
}

func TestATreeRunAsksTheAPIWhenGivenNoRefs(t *testing.T) {
	calls := 0
	srv := refsAPI(t, &calls)
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GITHUB_TOKEN", "tok")
	root := t.TempDir()
	dir := filepath.Join(root, ".github", "workflows")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ci.yml"), []byte("on: {push: {branches: ['**']}}\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: o/r@feature\n"), 0o644))

	var ids []string
	for _, f := range slopfix.CheckTreeWith(root, slopfix.Request{IDs: []string{workflow.IDBranchPin}}).Findings {
		ids = append(ids, f.ID)
	}
	assert.Equal(t, []string{workflow.IDBranchPin}, ids)
	assert.Positive(t, calls)
}

// cachedTree is a repository whose .git holds cache and whose workflow uses o/r@feature.
func cachedTree(t *testing.T, cache string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "slopfix-refs.json"), []byte(cache), 0o644))
	dir := filepath.Join(root, ".github", "workflows")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ci.yml"), []byte("on: {push: {branches: ['**']}}\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: o/r@feature\n"), 0o644))
	return root
}

// The API says feature is a branch. A cache that says it is a tag wins, and the API is not asked.
func TestARecordedAnswerIsUsedInPlaceOfTheAPI(t *testing.T) {
	calls := 0
	srv := refsAPI(t, &calls)
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GITHUB_TOKEN", "tok")
	root := cachedTree(t, `{"o/r":"master","o/r@feature":"tag"}`)

	assert.Empty(t, slopfix.CheckTreeWith(root, slopfix.Request{IDs: []string{workflow.IDBranchPin}}).Findings)
	assert.Zero(t, calls)
}

// The fixture API knows only what a fixture declares, so an undeclared repository is a finding and a declared one passes.
func TestAnUndeclaredFixtureRepositoryIsReported(t *testing.T) {
	text := "on: {push: {branches: ['**']}}\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: acme/undeclared@v1\n"
	judged := func(declared map[string]string) []slopfix.TreeFinding {
		c, err := slopfix.Materialize(t.TempDir(), slopfix.RuleCase{Path: ".github/workflows/ci.yml", Text: text, Refs: declared})
		require.NoError(t, err)
		return slopfix.CheckTreeWith(c.Root, slopfix.Request{IDs: []string{workflow.IDBranchPin}}).Findings
	}
	found := judged(nil)
	require.Len(t, found, 1)
	assert.Equal(t, "the branch pins could not be judged", found[0].Rule)
	assert.Empty(t, judged(map[string]string{"acme/undeclared": "master", "acme/undeclared@v1": "tag"}))
}

func TestAnUndeclaredFixtureRefIsReported(t *testing.T) {
	text := "on: {push: {branches: ['**']}}\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: acme/declared@v9\n"
	c, err := slopfix.Materialize(t.TempDir(), slopfix.RuleCase{Path: ".github/workflows/ci.yml", Text: text, Refs: map[string]string{"acme/declared": "master"}})
	require.NoError(t, err)
	found := slopfix.CheckTreeWith(c.Root, slopfix.Request{IDs: []string{workflow.IDBranchPin}}).Findings
	require.Len(t, found, 1)
	assert.Equal(t, workflow.IDBranchPin, found[0].ID)
}

// A ref the cache lacks is looked up, and the API's answer decides.
func TestAnUnrecordedRefIsLookedUp(t *testing.T) {
	calls := 0
	srv := refsAPI(t, &calls)
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GITHUB_TOKEN", "tok")
	root := cachedTree(t, `{"o/other":"master"}`)

	found := slopfix.CheckTreeWith(root, slopfix.Request{IDs: []string{workflow.IDBranchPin}}).Findings
	require.Len(t, found, 1)
	assert.Equal(t, "a uses: ref names a feature branch, which a merge deletes", found[0].Rule)
	assert.Positive(t, calls)
}

func TestARecordedBranchIsFlaggedWithoutTheAPI(t *testing.T) {
	t.Setenv("GITHUB_API_URL", "http://127.0.0.1:1")
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "slopfix-refs.json"), []byte(`{"o/r":"master","o/r@feature":"branch"}`), 0o644))
	dir := filepath.Join(root, ".github", "workflows")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ci.yml"), []byte("on: {push: {branches: ['**']}}\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: o/r@feature\n"), 0o644))

	found := slopfix.CheckTreeWith(root, slopfix.Request{IDs: []string{workflow.IDBranchPin}}).Findings
	require.Len(t, found, 1)
	assert.Equal(t, "a uses: ref names a feature branch, which a merge deletes", found[0].Rule)
}

func TestAnUnreadableRepositoryIsAFindingNotATag(t *testing.T) {
	calls := 0
	srv := refsAPI(t, &calls)
	env := map[string]string{"GITHUB_API_URL": srv.URL, "GITHUB_TOKEN": "tok"}
	refs := slopfix.NewGitHubRefs(func(k string) string { return env[k] })
	text := "on: {push: {branches: ['**']}}\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: o/private@v1\n"
	repair := slopfix.Report(slopfix.Request{Content: text, Path: ".github/workflows/ci.yml", Refs: refs, IDs: []string{workflow.IDBranchPin}})
	require.Len(t, repair.Findings, 1)
	assert.Equal(t, workflow.IDBranchPin, repair.Findings[0].ID)
	assert.Contains(t, repair.Findings[0].Detail, "404")
}

func TestFixWritesTheDefaultBranchOverABranchPin(t *testing.T) {
	calls := 0
	srv := refsAPI(t, &calls)
	env := map[string]string{"GITHUB_API_URL": srv.URL, "GITHUB_TOKEN": "tok"}
	refs := slopfix.NewGitHubRefs(func(k string) string { return env[k] })
	text := "on: {push: {branches: ['**']}}\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: o/r@feature\n"
	repair := slopfix.Fix(slopfix.Request{Content: text, Path: ".github/workflows/ci.yml", Refs: refs, IDs: []string{workflow.IDBranchPin}})
	assert.Contains(t, repair.Text, "uses: o/r@master")
	assert.Empty(t, repair.Findings)
}
