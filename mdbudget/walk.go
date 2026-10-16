package mdbudget

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// walker is a work queue of directories shared by a fixed pool of readers.
type walker struct {
	mu      sync.Mutex
	cond    *sync.Cond
	root    string
	queue   []string
	pending int
	found   []string
	old     walkCache
	fresh   walkCache
	racy    int64
}

func (w *walker) next() (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for len(w.queue) == 0 {
		if w.pending == 0 {
			return "", false
		}
		w.cond.Wait()
	}
	dir := w.queue[len(w.queue)-1]
	w.queue = w.queue[:len(w.queue)-1]
	return dir, true
}

func (w *walker) done(dir string, entry *dirEntry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	if entry != nil {
		w.fresh[w.key(dir)] = *entry
		for _, name := range entry.Subdirs {
			w.queue = append(w.queue, filepath.Join(dir, name))
		}
		n = len(entry.Subdirs)
		if entry.HasClaude {
			w.found = append(w.found, filepath.Join(dir, "CLAUDE.md"))
		}
	}
	w.pending += n - 1
	if n > 0 || w.pending == 0 {
		w.cond.Broadcast()
	}
}

// key names a directory in the cache by its path under the root.
func (w *walker) key(dir string) string {
	return dir[len(w.root):]
}

// read answers one directory from the cache when its mtime matches the
// cached one, and from ReadDir otherwise. Adding, removing or renaming an
// entry changes a directory's mtime, so a match means the listing is the same.
func (w *walker) read(dir string) {
	stat := os.Lstat
	if dir == w.root {
		stat = os.Stat // the root may be a symlink, such as /tmp on macOS
	}
	st, err := stat(dir)
	if err != nil || !st.IsDir() {
		w.done(dir, nil)
		return
	}
	mtime := st.ModTime().UnixNano()
	if cached, ok := w.old[w.key(dir)]; ok && cached.Mtime == mtime {
		w.done(dir, &cached)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		w.done(dir, nil)
		return
	}
	entry := dirEntry{Mtime: mtime}
	// A directory changed within the mtime resolution of the walk's start can
	// change again with no visible mtime change, so its entry never matches.
	if mtime >= w.racy {
		entry.Mtime = -1
	}
	if dir != w.root && isLinkedWorktree(dir, entries) {
		w.done(dir, &dirEntry{Mtime: entry.Mtime})
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			if name != ".git" && name != "node_modules" {
				entry.Subdirs = append(entry.Subdirs, name)
			}
			continue
		}
		if name == "CLAUDE.md" {
			entry.HasClaude = true
		}
	}
	w.done(dir, &entry)
}

// isLinkedWorktree reports whether dir is a git linked worktree: its .git is a
// file whose gitdir line names a directory inside a worktrees directory. A
// submodule's .git file names one inside a modules directory instead.
func isLinkedWorktree(dir string, entries []os.DirEntry) bool {
	for _, e := range entries {
		if e.Name() != ".git" || !e.Type().IsRegular() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, ".git"))
		if err != nil {
			return false
		}
		line, _, _ := strings.Cut(string(data), "\n")
		target, ok := strings.CutPrefix(strings.TrimSpace(line), "gitdir:")
		if !ok {
			return false
		}
		return filepath.Base(filepath.Dir(filepath.Clean(strings.TrimSpace(target)))) == "worktrees"
	}
	return false
}

// claudeMdFiles walks root recursively for every CLAUDE.md and returns them
// sorted. It prunes .git, node_modules and every git linked worktree below
// the root; the root itself is walked even when it is a worktree. A pool of
// readers shares the walk, and a cache of directory listings keyed by mtime
// spares the reads of directories that did not change. A pruned worktree is
// cached as a directory with nothing below it.
func claudeMdFiles(root string) []string {
	w := &walker{
		root:    root,
		queue:   []string{root},
		pending: 1,
		old:     loadWalkCache(root),
		fresh:   walkCache{},
		racy:    time.Now().Add(-2 * time.Second).UnixNano(),
	}
	w.cond = sync.NewCond(&w.mu)
	var wg sync.WaitGroup
	for range 4 * runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				dir, ok := w.next()
				if !ok {
					return
				}
				w.read(dir)
			}
		}()
	}
	wg.Wait()
	saveWalkCache(root, w.fresh)
	sort.Strings(w.found)
	return w.found
}
