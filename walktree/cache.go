// PersistFloor is the fewest directories a walk reads before its listings are kept across processes.

package walktree

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// PersistFloor is the fewest directories a walk reads before its listings are kept across processes.
var PersistFloor = 2000

// cachedDir is one listing as the cache file stores it: the names joined by
// NUL and the type bits beside them. A large tree decodes quickly.
type cachedDir struct {
	Rel    string
	Mtime  int64
	Pruned bool
	Names  string
	Modes  []uint32
}

// cachePath is the cache file for one root, or "" when the user has no cache
// directory.
func cachePath(root string) string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(root))
	return filepath.Join(base, "slopfix", "walktree", hex.EncodeToString(sum[:])[:16]+".gob")
}

// loadCache reads the listings kept for root. A missing or unreadable cache
// is an empty one, which makes the walk read every directory.
func loadCache(root string) map[string]*listing {
	out := map[string]*listing{}
	path := cachePath(root)
	if path == "" {
		return out
	}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	var dirs []cachedDir
	if err := gob.NewDecoder(f).Decode(&dirs); err != nil {
		return map[string]*listing{}
	}
	for _, d := range dirs {
		l := &listing{Mtime: d.Mtime, Pruned: d.Pruned}
		if d.Names != "" {
			names := strings.Split(d.Names, "\x00")
			if len(names) != len(d.Modes) {
				return map[string]*listing{}
			}
			l.Entries = make([]Entry, len(names))
			for i, name := range names {
				l.Entries[i] = Entry{Name: name, Mode: fs.FileMode(d.Modes[i])}
			}
		}
		out[d.Rel] = l
	}
	return out
}

// saveCache writes the listings of root through a temporary file and a
// rename. A reader sees the cache or the new one, never a torn one.
func saveCache(root string, dirs map[string]*listing) {
	if len(dirs) < PersistFloor {
		return
	}
	path := cachePath(root)
	if path == "" {
		return
	}
	out := make([]cachedDir, 0, len(dirs))
	for rel, l := range dirs {
		d := cachedDir{Rel: rel, Mtime: l.Mtime, Pruned: l.Pruned, Modes: make([]uint32, len(l.Entries))}
		names := make([]string, len(l.Entries))
		for i, e := range l.Entries {
			names[i] = e.Name
			d.Modes[i] = uint32(e.Mode)
		}
		d.Names = strings.Join(names, "\x00")
		out = append(out, d)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(path), "walk-*.tmp")
	if err != nil {
		return
	}
	tmp := f.Name()
	if err := gob.NewEncoder(f).Encode(out); err != nil {
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
