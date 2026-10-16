package forkscope

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noWait drops the pause between attempts for the length of a test.
func noWait(t *testing.T) {
	saved := listRetryWait
	listRetryWait = func(int) {}
	t.Cleanup(func() { listRetryWait = saved })
}

// A later attempt that answers decides the run, as it did when a CI check
// failed on one.
func TestAServerErrorOnTheListIsAskedAgain(t *testing.T) {
	noWait(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"o/fork": "https://github.com/up/stream"}`))
	}))
	defer srv.Close()
	got, err := listedUpstream("o/fork", srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/up/stream", got)
	assert.Equal(t, int32(2), calls.Load())
}

// A host that keeps failing fails the run, naming the status it answered.
func TestAListThatKeepsFailingFailsTheRun(t *testing.T) {
	noWait(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	_, err := listedUpstream("o/fork", srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}
