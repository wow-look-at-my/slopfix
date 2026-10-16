package slopfix

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wow-look-at-my/slopfix/workflow"
)

// refsFile is the file under a repository's .git that holds the ref lookups a fixture recorded.
const refsFile = "slopfix-refs.json"

// recordedRefs answers the lookups a fixture recorded, and asks next about
// the rest.
type recordedRefs struct {
	table map[string]string
	next  workflow.Refs
}

func (r recordedRefs) DefaultBranch(repo string) (string, error) {
	if branch, ok := r.table[repo]; ok {
		return branch, nil
	}
	if r.next == nil {
		return "", nil
	}
	return r.next.DefaultBranch(repo)
}

func (r recordedRefs) Kind(repo, ref string) (workflow.RefKind, error) {
	if kind, ok := r.table[repo+"@"+ref]; ok {
		if kind == "branch" {
			return workflow.RefBranch, nil
		}
		return workflow.RefTag, nil
	}
	if r.next == nil {
		return workflow.RefTag, nil
	}
	return r.next.Kind(repo, ref)
}

// fixtureRefs answers the lookups Materialize records for the files of a case:
// every remote uses: ref is a tag of a repository defaulting to master. This
// happens until the case names it.
func fixtureRefs(files map[string]string, named map[string]string) map[string]string {
	out := map[string]string{}
	for _, content := range files {
		for _, use := range workflow.Uses(content) {
			if _, ok := out[use.Repo]; !ok {
				out[use.Repo] = "master"
			}
			out[use.Repo+"@"+use.Ref] = "tag"
		}
	}
	for key, value := range named {
		out[key] = value
	}
	return out
}

// repoRootOf answers the nearest directory above path that holds a .git, or "".
func repoRootOf(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if dir == filepath.Dir(dir) {
			return ""
		}
	}
}

// recordRefs writes the lookups of a fixture under the repository at dir.
func recordRefs(dir string, refs map[string]string) error {
	data, err := json.Marshal(refs)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".git", refsFile), data, 0o644)
}

// refsAt answers the lookups for the repository at root: the ones recorded
// under it, and next for the rest. With nothing recorded it answers next.
func refsAt(root string, next workflow.Refs) (workflow.Refs, error) {
	if root == "" {
		return next, nil
	}
	data, err := os.ReadFile(filepath.Join(root, ".git", refsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return next, nil
	}
	if err != nil {
		return nil, err
	}
	table := map[string]string{}
	if err := json.Unmarshal(data, &table); err != nil {
		return nil, err
	}
	return recordedRefs{table: table, next: next}, nil
}

// pinsInTree is the branch-pin part of a walk of root. It judges the pins
// from the request's lookups and the ones recorded under root, repairing them
// when writing. A walk with neither judges none.
func pinsInTree(root string, req Request, writing bool) (repaired []string, found []TreeFinding, err error) {
	refs, err := refsAt(root, req.Refs)
	if err != nil || refs == nil {
		return nil, nil, err
	}
	if writing {
		if repaired, err = RepairBranchPinsTree(root, req, refs); err != nil {
			return nil, nil, err
		}
	}
	found, err = BranchPinsTree(root, req, refs)
	return repaired, found, err
}
