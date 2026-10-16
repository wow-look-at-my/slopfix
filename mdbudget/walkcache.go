package mdbudget

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"os"
	"path/filepath"
)

// dirEntry is one directory's listing as the walk needs it: the
// subdirectories it descends into and whether a CLAUDE.md sits here.
type dirEntry struct {
	Mtime     int64
	Subdirs   []string
	HasClaude bool
}

// walkCache maps a directory's path under the walk root to its listing.
type walkCache map[string]dirEntry

// walkCachePath is the cache file for one walk root, or "" when the user has
// no cache directory.
func walkCachePath(root string) string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(base, "slopfix", "mdbudget", hex.EncodeToString(sum[:])[:16]+".gob")
}

// loadWalkCache reads the cache for root. A missing or unreadable cache is an
// empty one, which makes the walk read every directory.
func loadWalkCache(root string) walkCache {
	path := walkCachePath(root)
	if path == "" {
		return walkCache{}
	}
	f, err := os.Open(path)
	if err != nil {
		return walkCache{}
	}
	defer f.Close()
	var c walkCache
	if err := gob.NewDecoder(f).Decode(&c); err != nil || c == nil {
		return walkCache{}
	}
	return c
}

// saveWalkCache writes the cache for root through a temporary file and a
// rename. A reader sees the cache or the new one, never a torn one.
func saveWalkCache(root string, c walkCache) {
	path := walkCachePath(root)
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(path), "walk-*.tmp")
	if err != nil {
		return
	}
	tmp := f.Name()
	if err := gob.NewEncoder(f).Encode(c); err != nil {
		f.Close()
		os.Remove(tmp)
		return
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
	}
}
