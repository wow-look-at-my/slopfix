package walktree

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Created records a file or directory this process made, so every walk that
// covers its directory lists it.
func Created(path string) {
	for _, t := range covering(path) {
		t.mu.Lock()
		t.created(path)
		t.mu.Unlock()
	}
}

// Removed records a file or directory this process deleted, so no walk lists
// it or anything under it.
func Removed(path string) {
	for _, t := range covering(path) {
		t.mu.Lock()
		t.removed(path)
		t.mu.Unlock()
	}
}

// covering answers each walk whose root holds path, with path made absolute.
func covering(path string) []*tree {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	memoMu.Lock()
	defer memoMu.Unlock()
	var out []*tree
	for root, t := range memo {
		if rel, ok := under(root, abs); ok && rel != "" {
			out = append(out, t)
		}
	}
	return out
}

// under answers path relative to root when root holds it.
func under(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	if rel == "." {
		return "", true
	}
	return rel, true
}

func (t *tree) created(path string) {
	abs, _ := filepath.Abs(path)
	rel, _ := under(t.root, abs)
	parts := strings.Split(rel, string(filepath.Separator))
	dir := ""
	for i, name := range parts {
		if pruned(name) {
			return
		}
		l := t.dirs[dir]
		if l == nil || l.Pruned {
			return
		}
		full := filepath.Join(t.root, join(dir, name))
		st, err := os.Lstat(full)
		if err != nil {
			return
		}
		if !hasEntry(l, name) {
			l.Entries = append(l.Entries, Entry{Name: name, Mode: st.Mode().Type()})
			sort.Slice(l.Entries, func(a, b int) bool { return l.Entries[a].Name < l.Entries[b].Name })
			l.Mtime = -1
		}
		dir = join(dir, name)
		if st.IsDir() && t.dirs[dir] == nil {
			t.dirs[dir] = &listing{Mtime: -1}
		}
		if i == len(parts)-1 || !st.IsDir() {
			return
		}
	}
}

func (t *tree) removed(path string) {
	abs, _ := filepath.Abs(path)
	rel, _ := under(t.root, abs)
	parent := filepath.Dir(rel)
	if parent == "." {
		parent = ""
	}
	if l := t.dirs[parent]; l != nil {
		name := filepath.Base(rel)
		kept := l.Entries[:0]
		for _, e := range l.Entries {
			if e.Name != name {
				kept = append(kept, e)
			}
		}
		l.Entries = kept
		l.Mtime = -1
	}
	prefix := rel + string(filepath.Separator)
	for key := range t.dirs {
		if key == rel || strings.HasPrefix(key, prefix) {
			delete(t.dirs, key)
		}
	}
}

func hasEntry(l *listing, name string) bool {
	for _, e := range l.Entries {
		if e.Name == name {
			return true
		}
	}
	return false
}
