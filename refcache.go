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

// recordedRefs answers the lookups a fixture recorded. A repository key holds its default branch.
type recordedRefs map[string]string

func (r recordedRefs) DefaultBranch(repo string) (string, error) { return r[repo], nil }

func (r recordedRefs) Kind(repo, ref string) (workflow.RefKind, error) {
	if r[repo+"@"+ref] == "branch" {
		return workflow.RefBranch, nil
	}
	return workflow.RefTag, nil
}

// recordRefs writes the lookups of a fixture under the repository at dir.
func recordRefs(dir string, refs map[string]string) error {
	data, err := json.Marshal(refs)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".git", refsFile), data, 0o644)
}

// recordedRefsAt answers the lookups recorded for the repository at root, and
// whether there are any.
func recordedRefsAt(root string) (workflow.Refs, bool, error) {
	data, err := os.ReadFile(filepath.Join(root, ".git", refsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	refs := recordedRefs{}
	if err := json.Unmarshal(data, &refs); err != nil {
		return nil, false, err
	}
	return refs, true, nil
}

// pinsInTree is the branch-pin part of a walk of root. A repository with
// recorded lookups gets its pins repaired when writing, and reported.
func pinsInTree(root string, req Request, writing bool) (repaired []string, found []TreeFinding, err error) {
	refs, recorded, err := recordedRefsAt(root)
	if err != nil || !recorded {
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
