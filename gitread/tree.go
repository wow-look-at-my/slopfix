package gitread

import (
	"bytes"
	"fmt"
	"github.com/wow-look-at-my/go-containers/set"
	"sort"
	"strings"
)

// Commit is the part of a commit object this package reads.
type Commit struct {
	OID     OID
	Tree    OID
	Parents []OID
}

// Commit answers a commit object.
func (r *Repo) Commit(oid OID) (*Commit, error) {
	obj, err := r.object(oid)
	if err != nil {
		return nil, err
	}
	if obj.typ != TypeCommit {
		return nil, fmt.Errorf("gitread: %s is a %s, not a commit", oid, obj.typ)
	}
	data := obj.data
	c := &Commit{OID: oid}
	for _, raw := range bytes.Split(data, []byte("\n")) {
		if len(raw) == 0 {
			break
		}
		switch {
		case bytes.HasPrefix(raw, []byte("tree ")):
			if c.Tree, err = ParseOID(string(raw[5:])); err != nil {
				return nil, err
			}
		case bytes.HasPrefix(raw, []byte("parent ")):
			p, err := ParseOID(string(raw[7:]))
			if err != nil {
				return nil, err
			}
			c.Parents = append(c.Parents, p)
		}
	}
	if c.Tree.IsZero() {
		return nil, fmt.Errorf("gitread: the commit %s names no tree", oid)
	}
	return c, nil
}

// PeelCommit follows tag objects until it reaches a commit.
func (r *Repo) PeelCommit(oid OID) (OID, error) {
	for i := 0; i < 10; i++ {
		obj, err := r.object(oid)
		if err != nil {
			return OID{}, err
		}
		switch obj.typ {
		case TypeCommit:
			return oid, nil
		case TypeTag:
			target, err := tagTarget(obj.data)
			if err != nil {
				return OID{}, err
			}
			oid = target
		default:
			return OID{}, fmt.Errorf("gitread: %s peels to a %s, not a commit", oid, obj.typ)
		}
	}
	return OID{}, fmt.Errorf("gitread: the tag %s does not peel to a commit", oid)
}

// tagTarget answers the object a tag names.
func tagTarget(data []byte) (OID, error) {
	for _, raw := range bytes.Split(data, []byte("\n")) {
		if len(raw) == 0 {
			break
		}
		if bytes.HasPrefix(raw, []byte("object ")) {
			return ParseOID(string(raw[7:]))
		}
	}
	return OID{}, fmt.Errorf("gitread: a tag names no object")
}

// TreeEntry is one entry of a tree.
type TreeEntry struct {
	Mode string
	OID  OID
}

// treeEntries reads the entries of a tree object.
func (r *Repo) treeEntries(oid OID) (map[string]TreeEntry, error) {
	obj, err := r.object(oid)
	if err != nil {
		return nil, err
	}
	if obj.typ != TypeTree {
		return nil, fmt.Errorf("gitread: %s is a %s, not a tree", oid, obj.typ)
	}
	data := obj.data
	out := map[string]TreeEntry{}
	for len(data) > 0 {
		sp := bytes.IndexByte(data, ' ')
		if sp < 0 {
			return nil, fmt.Errorf("gitread: a tree entry has no mode")
		}
		mode := string(data[:sp])
		data = data[sp+1:]
		nul := bytes.IndexByte(data, 0)
		if nul < 0 {
			return nil, fmt.Errorf("gitread: a tree entry has no name")
		}
		name := string(data[:nul])
		if len(data) < nul+1+20 {
			return nil, fmt.Errorf("gitread: a tree entry has no object name")
		}
		out[name] = TreeEntry{Mode: mode, OID: oidOf(data[nul+1 : nul+1+20])}
		data = data[nul+1+20:]
	}
	return out, nil
}

// TreeEntryAt answers the entry a path names in a tree, or false.
func (r *Repo) TreeEntryAt(commit OID, path string) (TreeEntry, bool, error) {
	commit, err := r.PeelCommit(commit)
	if err != nil {
		return TreeEntry{}, false, err
	}
	c, err := r.Commit(commit)
	if err != nil {
		return TreeEntry{}, false, err
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	oid := c.Tree
	for i, part := range parts {
		entries, err := r.treeEntries(oid)
		if err != nil {
			return TreeEntry{}, false, err
		}
		entry, ok := entries[part]
		if !ok {
			return TreeEntry{}, false, nil
		}
		if i == len(parts)-1 {
			return entry, true, nil
		}
		if entry.Mode != "40000" {
			return TreeEntry{}, false, nil
		}
		oid = entry.OID
	}
	return TreeEntry{}, false, nil
}

// TreeNames answers every blob path in a commit's tree, mapped to its entry.
// A submodule gitlink is skipped, because it names a commit elsewhere.
func (r *Repo) TreeNames(commit OID) (map[string]TreeEntry, error) {
	commit, err := r.PeelCommit(commit)
	if err != nil {
		return nil, err
	}
	c, err := r.Commit(commit)
	if err != nil {
		return nil, err
	}
	out := map[string]TreeEntry{}
	if err := r.flatten(c.Tree, "", out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repo) flatten(oid OID, prefix string, out map[string]TreeEntry) error {
	entries, err := r.treeEntries(oid)
	if err != nil {
		return err
	}
	for name, entry := range entries {
		path := name
		if prefix != "" {
			path = prefix + "/" + name
		}
		if entry.Mode == "40000" {
			if err := r.flatten(entry.OID, path, out); err != nil {
				return err
			}
			continue
		}
		if entry.Mode == "160000" {
			continue
		}
		out[path] = entry
	}
	return nil
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
	obj, err := r.object(entry.OID)
	if err != nil {
		return nil, err
	}
	if obj.typ != TypeBlob {
		return nil, fmt.Errorf("gitread: %s at %s is a %s, not a blob", path, commit, obj.typ)
	}
	return obj.data, nil
}

// Blob answers the content of a blob object.
func (r *Repo) Blob(oid OID) ([]byte, bool) {
	obj, err := r.object(oid)
	if err != nil || obj.typ != TypeBlob {
		return nil, false
	}
	return obj.data, true
}

// ChangedPaths answers every path that differs between commits, without
// rename detection, sorted.
func (r *Repo) ChangedPaths(from, to OID) ([]string, error) {
	a, err := r.TreeNames(from)
	if err != nil {
		return nil, err
	}
	b, err := r.TreeNames(to)
	if err != nil {
		return nil, err
	}
	seen := set.New[string]()
	for path, entry := range a {
		other, ok := b[path]
		if !ok || other.OID != entry.OID || other.Mode != entry.Mode {
			seen.Add(path)
		}
	}
	for path, entry := range b {
		if old, ok := a[path]; !ok || old.OID != entry.OID || old.Mode != entry.Mode {
			seen.Add(path)
		}
	}
	out := make([]string, 0, seen.Len())
	for path := range seen.All() {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}
