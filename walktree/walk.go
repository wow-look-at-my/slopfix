// Package walktree is the only code in slopfix that reads directories. A
// process walks each tree once. The first Get for a root walks it, and every
// later Get for that root, or for a directory inside it, answers from that
// walk. The walk prunes .git, node_modules and every git linked worktree below
// the root, and records every other entry. Each caller filters one listing
// rather than walking again. A cache of directory listings keyed by mtime,
// kept across processes, spares the reads of directories that did not change.
package walktree

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fstree "github.com/wow-look-at-my/go-fs-tree-fast"
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

// walker feeds fstree.Walk. Its read is the walk's ReadDir: it applies the
// cache and the pruning.
type walker struct {
	mu    sync.Mutex
	root  string
	old   map[string]*listing
	fresh map[string]*listing
	racy  int64
}

func walk(root string) *tree {
	w := &walker{
		root:  root,
		old:   loadCache(root),
		fresh: map[string]*listing{},
		racy:  time.Now().Add(-2 * time.Second).UnixNano(),
	}
	// A walk has no exclude patterns, so it cannot fail.
	_, _ = fstree.Walk(context.Background(), w, []string{root}, fstree.Options{
		Workers: 4 * runtime.NumCPU(),
		ReadDir: w.read,
	})
	for rel, l := range w.fresh {
		if l.Pruned {
			w.unlist(rel)
		}
	}
	saveCache(root, w.fresh)
	return &tree{root: root, dirs: w.fresh}
}

// key names a directory by its path under the root.
func (w *walker) key(dir string) string {
	return strings.TrimPrefix(strings.TrimPrefix(dir, w.root), string(filepath.Separator))
}

// listingOf answers the listing recorded for rel, making one when there is none.
func (w *walker) listingOf(rel string) *listing {
	l := w.fresh[rel]
	if l == nil {
		l = &listing{}
		w.fresh[rel] = l
	}
	return l
}

// AddDir records the entries the walk read in dir.
func (w *walker) AddDir(dir string, entries []fstree.Entry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	l := w.listingOf(w.key(dir))
	l.Entries = make([]Entry, len(entries))
	for i, e := range entries {
		l.Entries[i] = Entry{Name: e.Name, Mode: e.Mode}
	}
	sort.Slice(l.Entries, func(i, j int) bool { return l.Entries[i].Name < l.Entries[j].Name })
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
	p.Entries = kept
}

// record keeps a directory's mtime and pruning, and the entries when the
// sink will not see them.
func (w *walker) record(rel string, mtime int64, pruned bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	l := w.listingOf(rel)
	l.Mtime, l.Pruned = mtime, pruned
}

// read answers one directory from the cache when its mtime matches the
// cached one, and from os.ReadDir otherwise. Adding, removing or renaming an
// entry changes a directory's mtime, so a match means the listing is the same.
func (w *walker) read(dir string) ([]os.DirEntry, error) {
	rel := w.key(dir)
	stat := os.Lstat
	if rel == "" {
		stat = os.Stat // the root may be a symlink, such as /tmp on macOS
	}
	Ops.Add(1)
	st, err := stat(dir)
	if err != nil {
		return nil, err
	}
	mtime := st.ModTime().UnixNano()
	if cached := w.old[rel]; cached != nil && cached.Mtime == mtime {
		w.record(rel, mtime, cached.Pruned)
		return dirEntries(cached.Entries), nil
	}
	Ops.Add(1)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// A directory changed within the mtime resolution of the walk's start can change again unseen.
	if mtime >= w.racy {
		mtime = -1
	}
	if rel != "" && linkedWorktree(dir, entries) {
		w.record(rel, mtime, true)
		return nil, nil
	}
	w.record(rel, mtime, false)
	kept := entries[:0]
	for _, e := range entries {
		if !(e.IsDir() && pruned(e.Name())) {
			kept = append(kept, e)
		}
	}
	return kept, nil
}

// cachedEntry is a cached listing entry handed back to the walk as an os.DirEntry.
type cachedEntry struct{ Entry }

func (c cachedEntry) Name() string               { return c.Entry.Name }
func (c cachedEntry) IsDir() bool                { return c.Mode.IsDir() }
func (c cachedEntry) Type() fs.FileMode          { return c.Mode.Type() }
func (c cachedEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrInvalid }

func dirEntries(entries []Entry) []os.DirEntry {
	out := make([]os.DirEntry, len(entries))
	for i, e := range entries {
		out[i] = cachedEntry{e}
	}
	return out
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
