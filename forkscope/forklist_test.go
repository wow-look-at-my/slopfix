package forkscope

import _ "embed"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestABrokenForkListOrUpstreamFailsLoudly(t *testing.T) {
	untagged := t.TempDir()
	gitT(t, untagged, "init", "-q", "-b", "main")
	writeT(t, untagged, "other.md", "# Other\n")
	gitT(t, untagged, "add", "-A")
	gitT(t, untagged, "commit", "-q", "-m", "other")

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

// ghAnswering stands in for gh, recording the arguments of each call.
func ghAnswering(out string, err error, calls *[][]string) func(...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		*calls = append(*calls, args)
		return []byte(out), err
	}
}

// The parent's clone URL is built on GITHUB_SERVER_URL when an answer names
// the parent by full_name alone.
func TestWithNoTokenGHNamesTheParent(t *testing.T) {
	fx := newForkFixture(t)
	server := filepath.Dir(filepath.Dir(fx.parent))
	fullName := filepath.Base(filepath.Dir(fx.parent)) + "/parent"
	body, err := json.Marshal(map[string]any{"fork": true, "parent": map[string]any{"full_name": fullName, "default_branch": "main"}})
	require.NoError(t, err)
	var calls [][]string
	r := Resolver{
		Getenv:  envOf(map[string]string{"GITHUB_REPOSITORY": "o/fork", "GITHUB_SERVER_URL": "file://" + server}),
		ListURL: noList(t),
		GH:      ghAnswering(string(body), nil, &calls),
	}
	own, err := r.Lines(fx.fork)
	require.NoError(t, err)
	assert.Equal(t, []int{5, 7}, held(own, fx.fork, "doc.md", prose...))
	assert.Equal(t, [][]string{{"api", "graphql", "-f", "query=" + ghRepoQuery, "-F", "owner=o", "-F", "name=fork", "--jq", ghRepoShape}}, calls)
}

func TestWithNoTokenGHAnswersANonFork(t *testing.T) {
	var calls [][]string
	r := Resolver{
		Getenv:  envOf(map[string]string{"GITHUB_REPOSITORY": "o/plain"}),
		ListURL: noList(t),
		GH:      ghAnswering(`{"fork":false}`, nil, &calls),
	}
	own, err := r.Lines(t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, own)
	assert.Len(t, calls, 1)
}

func TestATokenOrAnAPIURLSkipsGH(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"token":   {"GITHUB_TOKEN": "tok"},
		"API URL": {},
	} {
		t.Run(name, func(t *testing.T) {
			env["GITHUB_REPOSITORY"] = "o/fork"
			env["GITHUB_API_URL"] = forkAPI(t, `{"fork":false}`).URL
			var calls [][]string
			r := Resolver{Getenv: envOf(env), ListURL: noList(t), GH: ghAnswering(`{}`, nil, &calls)}
			_, _ = r.Lines(t.TempDir())
			assert.Empty(t, calls)
		})
	}
}

func TestGHThatCannotAnswerFailsLoudly(t *testing.T) {
	cases := map[string]struct {
		out  string
		err  error
		want string
	}{
		"gh fails":          {err: fmt.Errorf("exit status 1: not logged in"), want: "gh api graphql for o/fork: exit status 1: not logged in"},
		"not JSON":          {out: "nope", want: "gh api graphql for o/fork: invalid character"},
		"no fork flag":      {out: `{"full_name":"o/fork"}`, want: "no fork flag"},
		"no parent":         {out: `{"fork":true}`, want: "names no parent"},
		"a parent unnamed":  {out: `{"fork":true,"parent":{"default_branch":"main"}}`, want: "names no parent"},
		"a parent unbranch": {out: `{"fork":true,"parent":{"full_name":"u/p"}}`, want: "names no parent"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			var calls [][]string
			r := Resolver{
				Getenv:  envOf(map[string]string{"GITHUB_REPOSITORY": "o/fork"}),
				ListURL: noList(t),
				GH:      ghAnswering(c.out, c.err, &calls),
			}
			_, err := r.Lines(t.TempDir())
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

func TestAForkOutsideAWorkTreeFailsLoudly(t *testing.T) {
	_, err := forkLines(t.TempDir(), forkEnv(forkAPI(t, forkBody("unused"))), noList(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rev-parse --show-toplevel")
}

//go:embed testdata/new-fork-fixture.md
var newForkFixtureMd string

//go:embed testdata/test-keep-takes-each-paired-line-alone.yml
var testKeepTakesEachPairedLineAloneYml string

//go:embed testdata/test-keep-takes-each-paired-line-alone-2.yml
var testKeepTakesEachPairedLineAloneYml2 string

//go:embed testdata/test-keep-takes-each-paired-line-alone-3.yml
var testKeepTakesEachPairedLineAloneYml3 string

//go:embed testdata/squash-fixture.md
var squashFixtureMd string

//go:embed testdata/squash-fixture-2.md
var squashFixtureMd2 string
