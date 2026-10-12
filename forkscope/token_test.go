package forkscope

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rateLimitedAPI(t *testing.T, want string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+want {
			http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"fork":false}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The gh CLI's GH_TOKEN is the token a developer's shell carries, so the API
// call sends it when GITHUB_TOKEN is unset.
func TestTheAPICallSendsGHToken(t *testing.T) {
	srv := rateLimitedAPI(t, "gh-tok")
	r := Resolver{Getenv: envOf(map[string]string{"GITHUB_API_URL": srv.URL, "GH_TOKEN": "gh-tok"})}
	info, err := r.fetchRepo("o/fork")
	require.NoError(t, err)
	require.NotNil(t, info.Fork)
	assert.False(t, *info.Fork)
}

// GITHUB_TOKEN, the one Actions sets, wins over GH_TOKEN.
func TestTheAPICallPrefersGitHubToken(t *testing.T) {
	srv := rateLimitedAPI(t, "actions-tok")
	r := Resolver{Getenv: envOf(map[string]string{"GITHUB_API_URL": srv.URL, "GITHUB_TOKEN": "actions-tok", "GH_TOKEN": "gh-tok"})}
	_, err := r.fetchRepo("o/fork")
	require.NoError(t, err)
}

// An anonymous call the API refuses fails with the status, and says which
// variables would have carried a token.
func TestARefusedAnonymousCallNamesTheStatusAndTheToken(t *testing.T) {
	srv := rateLimitedAPI(t, "tok")
	r := Resolver{Getenv: envOf(map[string]string{"GITHUB_API_URL": srv.URL})}
	_, err := r.fetchRepo("o/fork")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403 Forbidden")
	assert.Contains(t, err.Error(), "API rate limit exceeded")
	assert.Contains(t, err.Error(), "GITHUB_TOKEN")
	assert.Contains(t, err.Error(), "GH_TOKEN")
}
