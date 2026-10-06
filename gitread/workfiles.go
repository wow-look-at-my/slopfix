package gitread

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ChangedNames answers every path that differs between a commit's tree and the
// work tree, and every untracked path git does not ignore.
func (r *Repo) ChangedNames(base OID) ([]string, error) {
	base, err := r.PeelCommit(base)
	if err != nil {
		return nil, err
	}
	baseNames, err := r.TreeNames(base)
	if err != nil {
		return nil, err
	}
	idx, err := r.Index()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	add := func(path string) {
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	tracked := idx.Tracked()
	for path, entry := range tracked {
		if entry.Mode == "160000" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.workTree, filepath.FromSlash(path)))
		before, inBase := baseNames[path]
		if err != nil || !inBase {
			add(path)
			continue
		}
		text, ok := r.Blob(before.OID)
		if !ok || !bytes.Equal(text, data) {
			add(path)
		}
	}
	untracked, err := r.Untracked()
	if err != nil {
		return nil, err
	}
	for _, path := range untracked {
		add(path)
	}
	sort.Strings(out)
	return out, nil
}

// WorkFiles answers every path the index tracks, and every untracked path git
// does not ignore: what `git ls-files --cached --others --exclude-standard`
// prints, relative to the work tree. That tree is in slash form.
func (r *Repo) WorkFiles() ([]string, error) {
	idx, err := r.Index()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	tracked := idx.Tracked()
	for path := range tracked {
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	if r.workTree != "" {
		if err := r.walkUntracked(tracked, func(rel string) {
			if !seen[rel] {
				seen[rel] = true
				out = append(out, rel)
			}
		}); err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// Untracked answers every untracked path git does not ignore, in slash form.
func (r *Repo) Untracked() ([]string, error) {
	if r.workTree == "" {
		return nil, nil
	}
	idx, err := r.Index()
	if err != nil {
		return nil, err
	}
	var out []string
	if err := r.walkUntracked(idx.Tracked(), func(rel string) { out = append(out, rel) }); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// walkUntracked visits each untracked, non-ignored file under the work tree.
func (r *Repo) walkUntracked(tracked map[string]IndexEntry, visit func(string)) error {
	ignore := r.Ignore()
	return filepath.WalkDir(r.workTree, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(r.workTree, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if path == r.workTree || rel == ".git" || strings.HasPrefix(rel, ".git/") {
				return nil
			}
			if entry, ok := tracked[rel]; ok && entry.Mode == "160000" {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
				return filepath.SkipDir
			}
			if ignore.Ignored(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := tracked[rel]; ok {
			return nil
		}
		if ignore.Ignored(rel, false) {
			return nil
		}
		visit(rel)
		return nil
	})
}
