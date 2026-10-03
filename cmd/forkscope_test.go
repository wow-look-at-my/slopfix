package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
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
