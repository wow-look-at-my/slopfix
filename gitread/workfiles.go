package gitread

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// WorkFiles answers every path the index tracks, and every untracked path git
// does not ignore: what git ls-files --cached --others --exclude-standard
// prints, in slash form. An untracked repository inside the tree is one entry
// that ends in a slash.
func (r *Repo) WorkFiles() ([]string, error) {
	idx, err := r.Index()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(idx.byPath))
	for path := range idx.byPath {
		out = append(out, path)
	}
	untracked, err := r.untracked(idx)
	if err != nil {
		return nil, err
	}
	out = append(out, untracked...)
	sort.Strings(out)
	return out, nil
}

// Untracked answers every untracked path git does not ignore, in slash form.
func (r *Repo) Untracked() ([]string, error) {
	idx, err := r.Index()
	if err != nil {
		return nil, err
	}
	out, err := r.untracked(idx)
	sort.Strings(out)
	return out, err
}

// trackedDirs answers every directory that holds a tracked path.
func trackedDirs(idx *Index) set.Set[string] {
	dirs := set.New[string]()
	for _, e := range idx.entries {
		for dir := dirOf(e.Path); dir != "" && !dirs.Contains(dir); dir = dirOf(dir) {
			dirs.Add(dir)
		}
	}
	return dirs
}

// untracked walks the work tree. It skips the directories git skips: an
// ignored one, a submodule, and a nested repository, which it names once.
func (r *Repo) untracked(idx *Index) ([]string, error) {
	if r.workTree == "" {
		return nil, nil
	}
	ignore := r.Ignore()
	holding := trackedDirs(idx)
	var out []string
	err := filepath.WalkDir(r.workTree, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == r.workTree {
			return nil
		}
		if d.Name() == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(path[len(r.workTree)+1:])
		if d.IsDir() {
			if e, ok := idx.byPath[rel]; ok && e.IsGitlink() {
				return filepath.SkipDir
			}
			if !holding.Contains(rel) {
				if _, err := os.Lstat(filepath.Join(path, ".git")); err == nil {
					if !ignore.Ignored(rel, true) {
						out = append(out, rel+"/")
					}
					return filepath.SkipDir
				}
				if ignore.Ignored(rel, true) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if _, ok := idx.byPath[rel]; ok {
			return nil
		}
		if !ignore.Ignored(rel, false) {
			out = append(out, rel)
		}
		return nil
	})
	return out, err
}

// workOID answers the name git would give the work tree copy of an entry, and
// false when the file is gone. A file whose stat data matches the index, and
// was not written in the same instant as the index, keeps the index's name.
func (r *Repo) workOID(idx *Index, e IndexEntry, filemode bool) (OID, uint32, bool) {
	if e.unstaged {
		return e.OID, e.mode, true
	}
	path := filepath.Join(r.workTree, filepath.FromSlash(e.Path))
	info, err := os.Lstat(path)
	if err != nil {
		return OID{}, 0, false
	}
	mode := uint32(modeFile)
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		mode = modeSymlink
	case !info.Mode().IsRegular():
		return OID{}, 0, false
	case filemode && info.Mode()&0o111 != 0:
		mode = modeExec
	case !filemode && e.mode == modeExec:
		// With core.fileMode off, git keeps the mode the index records.
		mode = modeExec
	}
	mtime := info.ModTime().UnixNano()
	if mode == e.mode && uint32(info.Size()) == e.size && mtime == e.mtimeNS && mtime < idx.mtimeNS {
		return e.OID, mode, true
	}
	var data []byte
	if mode == modeSymlink {
		target, err := os.Readlink(path)
		if err != nil {
			return OID{}, 0, false
		}
		data = []byte(filepath.ToSlash(target))
	} else if data, err = os.ReadFile(path); err != nil {
		return OID{}, 0, false
	}
	return HashBlob(data), mode, true
}

// ChangedNames answers every path git diff --name-only names between a
// commit's tree and the work tree, and every untracked path git does not
// ignore, sorted. A submodule is left out.
func (r *Repo) ChangedNames(base OID) ([]string, error) {
	changed, untracked, err := r.Changes(base)
	if err != nil {
		return nil, err
	}
	all := set.Of(changed...)
	for _, path := range untracked {
		all.Add(strings.TrimSuffix(path, "/"))
	}
	out := all.Values()
	sort.Strings(out)
	return out, nil
}

// Changes answers, sorted, each path git diff --name-only names between a
// commit's tree and the work tree, and each untracked path git does not
// ignore. A submodule is left out.
func (r *Repo) Changes(base OID) (changed, untracked []string, err error) {
	baseNames, err := r.TreeNames(base)
	if err != nil {
		return nil, nil, err
	}
	idx, err := r.Index()
	if err != nil {
		return nil, nil, err
	}
	diff := set.New[string]()
	filemode := r.ConfigBool("core.filemode", true)
	for _, e := range idx.entries {
		if e.IsGitlink() {
			continue
		}
		if e.Stage != 0 {
			diff.Add(e.Path)
			continue
		}
		before, inBase := baseNames[e.Path]
		oid, mode, present := r.workOID(idx, e, filemode)
		if !inBase || !present || oid != before.OID || modeString(mode) != before.Mode {
			diff.Add(e.Path)
		}
	}
	for path := range baseNames {
		if _, tracked := idx.byPath[path]; !tracked {
			diff.Add(path)
		}
	}
	if untracked, err = r.untracked(idx); err != nil {
		return nil, nil, err
	}
	changed = diff.Values()
	sort.Strings(changed)
	sort.Strings(untracked)
	return changed, untracked, nil
}

// WorkChanged reports whether the work tree copy of a tracked path differs
// from the blob the index stages, and false for a path the index lacks.
func (r *Repo) WorkChanged(path string) bool {
	idx, err := r.Index()
	if err != nil {
		return false
	}
	e, ok := idx.byPath[path]
	if !ok || e.IsGitlink() {
		return false
	}
	oid, mode, present := r.workOID(idx, e, r.ConfigBool("core.filemode", true))
	return !present || oid != e.OID || mode != e.mode
}

// modeString spells a mode as a tree entry does.
func modeString(mode uint32) string {
	var buf [12]byte
	i := len(buf)
	for {
		i--
		buf[i] = byte('0' + mode&7)
		mode >>= 3
		if mode == 0 {
			break
		}
	}
	return string(buf[i:])
}

// Submodule opens the checkout of a submodule at a path relative to the work
// tree, or nil when it has none.
func (r *Repo) Submodule(rel string) (*Repo, error) {
	dir := filepath.Join(r.workTree, filepath.FromSlash(rel))
	if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
		return nil, nil
	}
	sub, err := OpenWorkTree(dir)
	if err != nil || sub == nil || sub.workTree != resolved(dir) {
		return nil, err
	}
	return sub, nil
}

// TrackedRecursive answers every path the index tracks, and each path a
// checked-out submodule's index tracks, prefixed with the submodule path: what
// git ls-files --cached --recurse-submodules prints.
func (r *Repo) TrackedRecursive() ([]string, error) {
	idx, err := r.Index()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range idx.entries {
		if !e.IsGitlink() {
			out = append(out, e.Path)
			continue
		}
		sub, err := r.Submodule(e.Path)
		if err != nil {
			return nil, err
		}
		if sub == nil {
			continue
		}
		nested, err := sub.TrackedRecursive()
		if err != nil {
			return nil, err
		}
		for _, path := range nested {
			out = append(out, e.Path+"/"+path)
		}
	}
	return out, nil
}
