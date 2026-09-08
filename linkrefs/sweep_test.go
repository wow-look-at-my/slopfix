package linkrefs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A session that dies mid-message leaves its fence marker behind. Without a
// sweep the temp directory grows without bound, a file per dead message.
func TestASweepCollectsAStrandedMarker(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	stale := filepath.Join(os.TempDir(), "slopfix-linkrefs-deadbeefdeadbeef")
	require.NoError(t, os.WriteFile(stale, nil, 0o600))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))

	// A marker written just now belongs to a message still in flight.
	live := filepath.Join(os.TempDir(), "slopfix-linkrefs-0123456789abcdef")
	require.NoError(t, os.WriteFile(live, nil, 0o600))

	sweepFences()

	assert.NoFileExists(t, stale)
	assert.FileExists(t, live, "a live message's marker is younger than the cutoff")
}

// The final flush is where the sweep runs, so a dead message's marker is
// collected by the next message that finishes.
func TestTheFinalFlushSweeps(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	stale := filepath.Join(os.TempDir(), "slopfix-linkrefs-feedfacefeedface")
	require.NoError(t, os.WriteFile(stale, nil, 0o600))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))

	clearFence("some-message", true)
	assert.NoFileExists(t, stale)
}

// A file the sweep does not own is left where it is.
func TestTheSweepTouchesNothingElse(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())

	other := filepath.Join(os.TempDir(), "something-else")
	require.NoError(t, os.WriteFile(other, nil, 0o600))
	old := time.Now().Add(-99 * time.Hour)
	require.NoError(t, os.Chtimes(other, old, old))

	sweepFences()
	assert.FileExists(t, other)
}
