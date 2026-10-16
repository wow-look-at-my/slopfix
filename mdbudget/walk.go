package mdbudget

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
)

// walker is a work queue of directories shared by a fixed pool of readers.
type walker struct {
	mu      sync.Mutex
	cond    *sync.Cond
	queue   []string
	pending int
	found   []string
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

func (w *walker) done(dirs, files []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queue = append(w.queue, dirs...)
	w.pending += len(dirs) - 1
	w.found = append(w.found, files...)
	if len(dirs) > 0 || w.pending == 0 {
		w.cond.Broadcast()
	}
}

func (w *walker) read(dir string) {
	var dirs, files []string
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				if name != ".git" && name != "node_modules" {
					dirs = append(dirs, filepath.Join(dir, name))
				}
				continue
			}
			if name == "CLAUDE.md" {
				files = append(files, filepath.Join(dir, name))
			}
		}
	}
	w.done(dirs, files)
}

// claudeMdFiles walks root recursively for every CLAUDE.md, skipping .git and
// node_modules, and returns them sorted. A pool of readers shares the walk.
// SessionStart's census, PostToolUse/Stop's change tracking and CI's
// full_scan all use this walk, so none can miss a file deep under the root.
func claudeMdFiles(root string) []string {
	w := &walker{queue: []string{root}, pending: 1}
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
	sort.Strings(w.found)
	return w.found
}
