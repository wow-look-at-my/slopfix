package forkscope

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitUsesCutsEachPart(t *testing.T) {
	cases := map[string][4]string{
		"owner/repo@v1":          {"owner/repo", "", "v1", "true"},
		"owner/repo/sub/dir@abc": {"owner/repo", "sub/dir", "abc", "true"},
		"owner/repo":             {"", "", "", "false"},
		"owner@v1":               {"", "", "", "false"},
		"owner/repo@":            {"", "", "", "false"},
	}
	for uses, want := range cases {
		repo, path, ref, ok := splitUses(uses)
		assert.Equal(t, want, [4]string{repo, path, ref, map[bool]string{true: "true", false: "false"}[ok]}, uses)
	}
}

// Any other failure is an error.
func TestHasTagReadsA404AsNoTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/tagged/git/ref/tags/lkgb":
			fmt.Fprint(w, `{"ref":"refs/tags/lkgb"}`)
		case "/repos/o/broken/git/ref/tags/lkgb":
			http.Error(w, "boom", http.StatusBadGateway)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	r := Resolver{Getenv: envOf(map[string]string{"GITHUB_API_URL": srv.URL, "GITHUB_TOKEN": "tok"})}
	has, err := r.HasTag("o/tagged", "lkgb")
	require.NoError(t, err)
	assert.True(t, has)
	has, err = r.HasTag("o/untagged", "lkgb")
	require.NoError(t, err)
	assert.False(t, has)
	_, err = r.HasTag("o/broken", "lkgb")
	require.Error(t, err)

	var calls [][]string
	gh := Resolver{Getenv: envOf(nil), GH: ghAnswering("", errors.New("exit status 1: gh: Not Found (HTTP 404)"), &calls)}
	has, err = gh.HasTag("o/gh-untagged", "lkgb")
	require.NoError(t, err)
	assert.False(t, has)
	gh = Resolver{Getenv: envOf(nil), GH: ghAnswering("", errors.New("exit status 1: gh: Bad Gateway (HTTP 502)"), &calls)}
	_, err = gh.HasTag("o/gh-broken", "lkgb")
	require.Error(t, err)
}
