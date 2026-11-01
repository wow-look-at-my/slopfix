package slopfix

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func swapGHAPI(t *testing.T, fake func(path string) ([]byte, error)) *[]string {
	t.Helper()
	t.Serial()
	var calls []string
	prev := ghAPI
	ghAPI = func(path string) ([]byte, error) {
		calls = append(calls, path)
		return fake(path)
	}
	t.Cleanup(func() { ghAPI = prev })
	return &calls
}

func noEnv(string) string { return "" }

func TestParseIncluded_ReadsTheStatusAndTheBody(t *testing.T) {
	status, body, ok := parseIncluded([]byte("HTTP/2.0 404 Not Found\r\nX-A: b\r\n\r\n{\"message\":\"Not Found\"}"))
	require.True(t, ok)
	assert.Equal(t, 404, status)
	assert.Equal(t, `{"message":"Not Found"}`, string(body))
	status, body, ok = parseIncluded([]byte("HTTP/2.0 200 OK\nA: b\n\n{}\n"))
	require.True(t, ok)
	assert.Equal(t, 200, status)
	assert.Equal(t, "{}\n", string(body))
	_, _, ok = parseIncluded([]byte("gh: not logged in\n"))
	assert.False(t, ok, "output with no status line is no answer")
}

func TestGithubGet_AsksGHWhenNoTokenIsSet(t *testing.T) {
	calls := swapGHAPI(t, func(string) ([]byte, error) {
		return []byte("HTTP/2.0 200 OK\r\n\r\n{\"default_branch\":\"main\"}"), nil
	})
	status, body, err := githubGet(noEnv, "/repos/o/r")
	require.NoError(t, err)
	assert.Equal(t, 200, status)
	assert.JSONEq(t, `{"default_branch":"main"}`, string(body))
	assert.Equal(t, []string{"/repos/o/r"}, *calls)
}

func TestGithubGet_KeepsTheTokenAndTheAPIURLPaths(t *testing.T) {
	calls := swapGHAPI(t, func(string) ([]byte, error) { return nil, errors.New("unused") })
	for _, env := range []map[string]string{
		{"GITHUB_TOKEN": "t", "GITHUB_API_URL": "http://127.0.0.1:1"},
		{"GH_TOKEN": "t", "GITHUB_API_URL": "http://127.0.0.1:1"},
		{"GITHUB_API_URL": "http://127.0.0.1:1"},
	} {
		_, _, err := githubGet(func(k string) string { return env[k] }, "/repos/o/r")
		assert.Error(t, err, "the closed port answers nothing")
	}
	assert.Empty(t, *calls, "a token or an API URL never goes through gh")
}

func TestGithubGetViaGH_NoAnswerFallsBack(t *testing.T) {
	swapGHAPI(t, func(string) ([]byte, error) { return nil, errors.New("gh: command not found") })
	_, _, ok := githubGetViaGH("/repos/o/r")
	assert.False(t, ok)
}
