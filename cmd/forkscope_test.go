package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix"
)

// A test that walks a directory through runCheck reads os.Getenv.
func TestMain(m *testing.M) {
	if err := os.Unsetenv("GITHUB_REPOSITORY"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// A branch pin fails the tree check, and a lookup that cannot be done is the error.
func TestATreeCheckReportsABranchPin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r":
			_, _ = w.Write([]byte(`{"default_branch":"master"}`))
		case "/repos/o/r/git/ref/heads/feature":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	env := map[string]string{"GITHUB_API_URL": srv.URL}
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "action.yml"), []byte("runs:\n  using: composite\n  steps:\n    - uses: o/r@feature\n"), 0o644))
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	failed, err := treeFindings(cmd, root, slopfix.Request{}, false, func(k string) string { return env[k] })
	require.NoError(t, err)
	assert.True(t, failed)
	assert.Contains(t, out.String(), "yaml/branch-pin")

	require.NoError(t, os.WriteFile(filepath.Join(root, "action.yml"), []byte("runs:\n  using: composite\n  steps:\n    - uses: o/hidden@v1\n"), 0o644))
	_, err = treeFindings(cmd, root, slopfix.Request{}, false, func(k string) string { return env[k] })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "o/hidden")
}

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

	failed, err := treeFindings(cmd, t.TempDir(), slopfix.Request{}, false, func(k string) string { return env[k] })
	require.Error(t, err)
	assert.False(t, failed)
	assert.Contains(t, err.Error(), "500")
	assert.Empty(t, out.String())
}
