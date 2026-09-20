// Per-session state: which instruction files THIS session left too big, and
// what every candidate looked like last time we checked.
//
// Keyed per session so parallel sessions never collide, in tmp because it is
// turn-scoped state, not anything worth persisting.

package mdbudget

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// marker is the session's accumulated view. Fired records the signature the
// Stop gate blocked on PER FILE, which is the no-wedge property.
// Candidates is the walk's result, kept so a tool call costs a stat per
// instruction file rather than a traversal of the whole tree. Finding the
// candidates is what is expensive; measuring them is not.
type marker struct {
	Paths      []string          `json:"paths"`
	Fired      map[string]string `json:"fired"`
	Seen       map[string]string `json:"seen"`
	Candidates []string          `json:"candidates"`
	WalkedAt   int64             `json:"walked_at"`
}

func newMarker() *marker {
	return &marker{Fired: map[string]string{}, Seen: map[string]string{}}
}

func markerPath(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return filepath.Join(os.TempDir(), "claude-md-budget", hex.EncodeToString(sum[:])[:16]+".json")
}

func readMarker(sessionID string) *marker {
	data, err := os.ReadFile(markerPath(sessionID))
	if err != nil {
		return nil
	}
	var m marker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	if m.Fired == nil {
		m.Fired = map[string]string{}
	}
	if m.Seen == nil {
		m.Seen = map[string]string{}
	}
	return &m
}

// writeMarker persists the marker. Losing it costs a nag, not correctness.
func writeMarker(sessionID string, m *marker) {
	path := markerPath(sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func recordOffender(sessionID, path string) {
	if sessionID == "" {
		return
	}
	m := readMarker(sessionID)
	if m == nil {
		m = newMarker()
	}
	for _, p := range m.Paths {
		if p == path {
			return
		}
	}
	m.Paths = append(m.Paths, path)
	writeMarker(sessionID, m)
}

// noteSignature records a single file's current signature, so the next call
// that diffs the snapshot does not report a write this already answered for.
func noteSignature(sessionID, path string) {
	if sessionID == "" {
		return
	}
	m := readMarker(sessionID)
	if m == nil {
		m = newMarker()
	}
	if sig, ok := signature(path); ok {
		m.Seen[path] = sig
	}
	writeMarker(sessionID, m)
}

// seedSnapshot records what every candidate looked like BEFORE this session
// touched anything, or the earliest Bash-written edit gets away.
func seedSnapshot(sessionID, cwd string) {
	if sessionID == "" {
		return
	}
	m := readMarker(sessionID)
	if m == nil {
		m = newMarker()
	}
	m.Candidates = allCandidatePaths(cwd)
	m.WalkedAt = time.Now().Unix()
	m.Seen = snapshotOf(m.Candidates)
	writeMarker(sessionID, m)
}

// walkTTL is how long a candidate list stands before the tree is walked again.
// A CLAUDE.md that appears mid-session is reported within this, and every tool
// call inside it costs stats instead of a traversal.
const walkTTL = 120

// candidates answers the marker's list while it is fresh, and walks otherwise.
// It reports whether it walked, so the caller can persist what it found.
func candidates(m *marker, cwd string) ([]string, bool) {
	if len(m.Candidates) > 0 && time.Now().Unix()-m.WalkedAt < walkTTL {
		return m.Candidates, false
	}
	return allCandidatePaths(cwd), true
}

// snapshotOf reads a signature per path, keyed absolute so two spellings of one
// file count once.
func snapshotOf(paths []string) map[string]string {
	seen := map[string]string{}
	for _, path := range paths {
		key, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, done := seen[key]; done {
			continue
		}
		if sig, ok := signature(key); ok {
			seen[key] = sig
		}
	}
	return seen
}

// changedFiles refreshes the snapshot and returns the instruction files that
// CHANGED. Watching files rather than tool_input.file_path catches a Bash edit.
func changedFiles(sessionID, cwd string) []string {
	if sessionID == "" {
		return nil
	}
	m := readMarker(sessionID)
	if m == nil {
		m = newMarker()
	}
	first := len(m.Seen) == 0
	paths, walked := candidates(m, cwd)
	if walked {
		m.Candidates = paths
		m.WalkedAt = time.Now().Unix()
	}
	seen := snapshotOf(paths)

	var changed []string
	if !first {
		for key, sig := range seen {
			if m.Seen[key] != sig {
				changed = append(changed, key)
			}
		}
	}
	m.Seen = seen
	writeMarker(sessionID, m)
	return changed
}
