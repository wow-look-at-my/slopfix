// Package walktree is the only code in slopfix that reads directories. A
// process walks each tree once. The first Get for a root walks it, and every
// later Get for that root, or for a directory inside it, answers from that
// walk. The walk prunes .git, node_modules and every git linked worktree below
// the root, and records every other entry. Each caller filters one listing
// rather than walking again. A cache of directory listings keyed by mtime,
// kept across processes, spares the reads of directories that did not change.
package walktree

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Entry is one name in a directory and its type bits.
type Entry struct {
	Name string
	Mode fs.FileMode
}

// IsDir reports whether the entry is a directory.
func (e Entry) IsDir() bool { return e.Mode.IsDir() }

// IsRegular reports whether the entry is a regular file.
func (e Entry) IsRegular() bool { return e.Mode.IsRegular() }

// listing is one directory as the walk read it.
type listing struct {
	Mtime   int64
	Entries []Entry
	Pruned  bool
}

// Ops counts the directory reads and stats the walk performs.
var Ops atomic.Int64

// tree is one walked root. dirs is keyed by the path under the root, with
// "" for the root itself.
type tree struct {
	mu   sync.Mutex
	root string
	dirs map[string]*listing
}

var (
	memoMu sync.Mutex
	memo   = map[string]*tree{}
)

// pruned reports a name the walk never descends into.
func pruned(name string) bool {
	return name == ".git" || name == "node_modules"
}

// View is a walked tree seen from one directory in it, spelled as the caller
// spelled it.
type View struct {
	t     *tree
	rel   string
	shown string
}

// Get answers the view of root, walking it only when no walk this process
// made covers it.
func Get(root string) View {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	memoMu.Lock()
	defer memoMu.Unlock()
	if t, ok := memo[abs]; ok {
		return View{t: t, shown: root}
	}
	for walked, t := range memo {
		rel, err := filepath.Rel(walked, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		t.mu.Lock()
		_, covered := t.dirs[rel]
		t.mu.Unlock()
		if covered {
			return View{t: t, rel: rel, shown: root}
		}
	}
	t := walk(abs)
	memo[abs] = t
	return View{t: t, shown: root}
}

// Forget drops every walk this process made.
func Forget() {
	memoMu.Lock()
	defer memoMu.Unlock()
	memo = map[string]*tree{}
}

// Visit calls enter for each directory below the view's root and file for
// each other entry, in the lexical order filepath.WalkDir uses.
func (v View) Visit(enter func(dir string) bool, file func(path string, e Entry)) {
	if v.t == nil {
		return
	}
	v.t.mu.Lock()
	defer v.t.mu.Unlock()
	v.visit(v.rel, v.shown, enter, file)
}

func (v View) visit(rel, shown string, enter func(string) bool, file func(string, Entry)) {
	l := v.t.dirs[rel]
	if l == nil {
		return
	}
	for _, e := range l.Entries {
		path := filepath.Join(shown, e.Name)
		if !e.IsDir() {
			file(path, e)
			continue
		}
		if enter != nil && !enter(path) {
			continue
		}
		v.visit(join(rel, e.Name), path, enter, file)
	}
}

// Files answers every entry that is not a directory, in walk order.
func (v View) Files() []string {
	var out []string
	v.Visit(nil, func(path string, _ Entry) { out = append(out, path) })
	return out
}

// Named answers every entry called name, in walk order.
func (v View) Named(name string) []string {
	var out []string
	v.Visit(nil, func(path string, e Entry) {
		if e.Name == name {
			out = append(out, path)
		}
	})
	return out
}

// List reads one directory, sorted by name. It does not recurse.
func List(dir string) ([]os.DirEntry, error) {
	Ops.Add(1)
	return os.ReadDir(dir)
}

func join(rel, name string) string {
	if rel == "" {
		return name
	}
	return filepath.Join(rel, name)
}

// walker is a work queue of directories shared by a fixed pool of readers.
// old is the cache the walk started from and is only read.
type walker struct {
	mu      sync.Mutex
	cond    *sync.Cond
	root    string
	queue   []string
	pending int
	old     map[string]*listing
	fresh   map[string]*listing
	racy    int64
}

func walk(root string) *tree {
	w := &walker{
		root:    root,
		queue:   []string{""},
		pending: 1,
		old:     loadCache(root),
		fresh:   map[string]*listing{},
		racy:    time.Now().Add(-2 * time.Second).UnixNano(),
	}
	w.cond = sync.NewCond(&w.mu)
	var wg sync.WaitGroup
	for range 4 * runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				rel, ok := w.next()
				if !ok {
					return
				}
				w.read(rel)
			}
		}()
	}
	wg.Wait()
	saveCache(root, w.fresh)
	return &tree{root: root, dirs: w.fresh}
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
	rel := w.queue[len(w.queue)-1]
	w.queue = w.queue[:len(w.queue)-1]
	return rel, true
}

func (w *walker) done(rel string, l *listing) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	if l != nil {
		w.fresh[rel] = l
		if l.Pruned {
			w.unlist(rel)
		}
		for _, e := range l.Entries {
			if e.IsDir() {
				w.queue = append(w.queue, join(rel, e.Name))
				n++
			}
		}
	}
	w.pending += n - 1
	if n > 0 || w.pending == 0 {
		w.cond.Broadcast()
	}
}

// unlist removes a pruned directory from its parent's listing.
func (w *walker) unlist(rel string) {
	parent := filepath.Dir(rel)
	if parent == "." {
		parent = ""
	}
	p := w.fresh[parent]
	if p == nil {
		return
	}
	name := filepath.Base(rel)
	kept := p.Entries[:0:0]
	for _, e := range p.Entries {
		if e.Name != name {
			kept = append(kept, e)
		}
	}
	w.fresh[parent] = &listing{Mtime: p.Mtime, Entries: kept}
}

// read answers one directory from the cache when its mtime matches the
// cached one, and from ReadDir otherwise. Adding, removing or renaming an
// entry changes a directory's mtime, so a match means the listing is the same.
func (w *walker) read(rel string) {
	dir := filepath.Join(w.root, rel)
	stat := os.Lstat
	if rel == "" {
		stat = os.Stat // the root may be a symlink, such as /tmp on macOS
	}
	Ops.Add(1)
	st, err := stat(dir)
	if err != nil || !st.IsDir() {
		w.done(rel, nil)
		return
	}
	mtime := st.ModTime().UnixNano()
	if cached := w.old[rel]; cached != nil && cached.Mtime == mtime {
		w.done(rel, cached)
		return
	}
	Ops.Add(1)
	entries, err := os.ReadDir(dir)
	if err != nil {
		w.done(rel, nil)
		return
	}
	l := &listing{Mtime: mtime}
	// A directory changed within the mtime resolution of the walk's start can
	// change again with no visible mtime change, so its listing never matches.
	if mtime >= w.racy {
		l.Mtime = -1
	}
	if rel != "" && linkedWorktree(dir, entries) {
		l.Pruned = true
		w.done(rel, l)
		return
	}
	for _, e := range entries {
		if e.IsDir() && pruned(e.Name()) {
			continue
		}
		l.Entries = append(l.Entries, Entry{Name: e.Name(), Mode: e.Type()})
	}
	sort.Slice(l.Entries, func(i, j int) bool { return l.Entries[i].Name < l.Entries[j].Name })
	w.done(rel, l)
}

// linkedWorktree reports whether dir is a git linked worktree: its .git is a
// file whose gitdir line names a directory inside a worktrees directory. A
// submodule's .git file names one inside a modules directory instead.
func linkedWorktree(dir string, entries []os.DirEntry) bool {
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
