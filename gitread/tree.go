package gitread

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Tree entry modes, as a tree spells them.
const (
	treeModeDir     = "40000"
	treeModeGitlink = "160000"
)

// TreeEntry is one entry of a tree.
type TreeEntry struct {
	Mode string
	OID  OID
}

// IsDir reports a subtree.
func (e TreeEntry) IsDir() bool { return e.Mode == treeModeDir }

// namedEntry is a tree entry with its name, in tree order.
type namedEntry struct {
	name string
	TreeEntry
}

// tree answers the entries of a tree object, sorted by name.
func (r *Repo) tree(oid OID) ([]namedEntry, error) {
	if entries, ok := r.db.trees.get(oid); ok {
		return entries, nil
	}
	obj, err := r.db.find(oid, true)
	if err != nil {
		return nil, err
	}
	if obj.kind != kindTree {
		return nil, fmt.Errorf("gitread: %s is a %s, not a tree", oid, kindNames[obj.kind])
	}
	entries, err := parseTree(obj.data)
	if err != nil {
		return nil, err
	}
	r.db.trees.put(oid, entries, len(obj.data)+len(entries)*entryCost/2)
	return entries, nil
}

func parseTree(data []byte) ([]namedEntry, error) {
	var out []namedEntry
	for len(data) > 0 {
		sp := bytes.IndexByte(data, ' ')
		if sp < 0 {
			return nil, fmt.Errorf("gitread: a tree entry has no mode")
		}
		mode := string(data[:sp])
		data = data[sp+1:]
		nul := bytes.IndexByte(data, 0)
		if nul < 0 || len(data) < nul+1+20 {
			return nil, fmt.Errorf("gitread: a tree entry is truncated")
		}
		out = append(out, namedEntry{string(data[:nul]), TreeEntry{Mode: mode, OID: oidOf(data[nul+1 : nul+21])}})
		data = data[nul+21:]
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}

// lookup finds a name among sorted entries.
func lookup(entries []namedEntry, name string) (TreeEntry, bool) {
	i := sort.Search(len(entries), func(i int) bool { return entries[i].name >= name })
	if i < len(entries) && entries[i].name == name {
		return entries[i].TreeEntry, true
	}
	return TreeEntry{}, false
}

// rootTree answers the tree a commit or tag names.
func (r *Repo) rootTree(commit OID) (OID, error) {
	commit, err := r.PeelCommit(commit)
	if err != nil {
		return OID{}, err
	}
	c, err := r.Commit(commit)
	if err != nil {
		return OID{}, err
	}
	return c.Tree, nil
}

// TreeEntryAt answers the entry a path names at a commit, or false.
func (r *Repo) TreeEntryAt(commit OID, path string) (TreeEntry, bool, error) {
	oid, err := r.rootTree(commit)
	if err != nil {
		return TreeEntry{}, false, err
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		entries, err := r.tree(oid)
		if err != nil {
			return TreeEntry{}, false, err
		}
		entry, ok := lookup(entries, part)
		if !ok {
			return TreeEntry{}, false, nil
		}
		if i == len(parts)-1 {
			return entry, true, nil
		}
		if !entry.IsDir() {
			return TreeEntry{}, false, nil
		}
		oid = entry.OID
	}
	return TreeEntry{}, false, nil
}

// TreeNames answers every file path in a commit's tree, mapped to its entry.
// A submodule gitlink is left out, because it names a commit elsewhere.
func (r *Repo) TreeNames(commit OID) (map[string]TreeEntry, error) {
	oid, err := r.rootTree(commit)
	if err != nil {
		return nil, err
	}
	out := map[string]TreeEntry{}
	err = r.walkTree(oid, "", func(path string, e TreeEntry) {
		if e.Mode != treeModeGitlink {
			out[path] = e
		}
	})
	return out, err
}

// walkTree visits every non-tree entry under a tree.
func (r *Repo) walkTree(oid OID, prefix string, visit func(string, TreeEntry)) error {
	entries, err := r.tree(oid)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := joinPath(prefix, e.name)
		if e.IsDir() {
			if err := r.walkTree(e.OID, path, visit); err != nil {
				return err
			}
			continue
		}
		visit(path, e.TreeEntry)
	}
	return nil
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

// BlobAt answers the content of the blob a path names at a commit.
func (r *Repo) BlobAt(commit OID, path string) ([]byte, error) {
	entry, ok, err := r.TreeEntryAt(commit, path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("gitread: %s is not in the tree of %s", path, commit)
	}
	obj, err := r.db.object(entry.OID)
	if err != nil {
		return nil, err
	}
	if obj.kind != kindBlob {
		return nil, fmt.Errorf("gitread: %s at %s is a %s, not a blob", path, commit, kindNames[obj.kind])
	}
	return obj.data, nil
}

// Blob answers the content of a blob object.
func (r *Repo) Blob(oid OID) ([]byte, bool) {
	obj, err := r.db.object(oid)
	if err != nil || obj.kind != kindBlob {
		return nil, false
	}
	return obj.data, true
}

// DiffTrees visits each path whose entry differs between commits, without
// rename detection. A side that lacks the path passes nil. It reads a subtree
// only where both sides name different subtrees.
func (r *Repo) DiffTrees(from, to OID, visit func(path string, old, new *TreeEntry)) error {
	var a, b OID
	var err error
	if !from.IsZero() {
		if a, err = r.rootTree(from); err != nil {
			return err
		}
	}
	if b, err = r.rootTree(to); err != nil {
		return err
	}
	return r.diffTree(a, b, "", visit)
}

func (r *Repo) entriesOf(oid OID) ([]namedEntry, error) {
	if oid.IsZero() {
		return nil, nil
	}
	return r.tree(oid)
}

func (r *Repo) diffTree(a, b OID, prefix string, visit func(string, *TreeEntry, *TreeEntry)) error {
	if a == b {
		return nil
	}
	left, err := r.entriesOf(a)
	if err != nil {
		return err
	}
	right, err := r.entriesOf(b)
	if err != nil {
		return err
	}
	i, j := 0, 0
	for i < len(left) || j < len(right) {
		switch {
		case j == len(right) || i < len(left) && left[i].name < right[j].name:
			if err := r.diffSide(left[i], prefix, true, visit); err != nil {
				return err
			}
			i++
		case i == len(left) || right[j].name < left[i].name:
			if err := r.diffSide(right[j], prefix, false, visit); err != nil {
				return err
			}
			j++
		default:
			l, rt := left[i], right[j]
			path := joinPath(prefix, l.name)
			switch {
			case l.TreeEntry == rt.TreeEntry:
			case l.IsDir() && rt.IsDir():
				if err := r.diffTree(l.OID, rt.OID, path, visit); err != nil {
					return err
				}
			case l.IsDir():
				if err := r.diffSide(l, prefix, true, visit); err != nil {
					return err
				}
				visit(path, nil, &rt.TreeEntry)
			case rt.IsDir():
				visit(path, &l.TreeEntry, nil)
				if err := r.diffSide(rt, prefix, false, visit); err != nil {
					return err
				}
			default:
				visit(path, &l.TreeEntry, &rt.TreeEntry)
			}
			i++
			j++
		}
	}
	return nil
}

// diffSide visits an entry only one side holds, every file of it for a tree.
func (r *Repo) diffSide(e namedEntry, prefix string, old bool, visit func(string, *TreeEntry, *TreeEntry)) error {
	emit := func(path string, entry TreeEntry) {
		if old {
			visit(path, &entry, nil)
		} else {
			visit(path, nil, &entry)
		}
	}
	path := joinPath(prefix, e.name)
	if !e.IsDir() {
		emit(path, e.TreeEntry)
		return nil
	}
	return r.walkTree(e.OID, path, emit)
}

// ChangedPaths answers every path that differs between commits, sorted.
func (r *Repo) ChangedPaths(from, to OID) ([]string, error) {
	var out []string
	err := r.DiffTrees(from, to, func(path string, _, _ *TreeEntry) { out = append(out, path) })
	sort.Strings(out)
	return out, err
}
