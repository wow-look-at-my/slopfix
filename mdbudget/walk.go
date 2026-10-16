package mdbudget

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
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

// claudeMdFiles walks root recursively for every CLAUDE.md, skipping .git and
// node_modules, and returns them sorted. A pool of readers shares the walk,
// and a cache of directory listings keyed by mtime spares the reads of
// directories that did not change. SessionStart's census, PostToolUse/Stop's
// change tracking and CI's full_scan all use this walk, so none can miss a
// file deep under the root.
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
