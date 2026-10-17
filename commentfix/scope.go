// scope.go decides which files a build sweep may write.
//
// A build runs the sweep on every tree it touches, and a rewrite there lands
// in the author's working copy. So the sweep writes only what the branch
// already changed, and nothing while git is part way through a merge or a
// rebase. A rewrite then mixes into the conflict resolution.
package commentfix

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitread"
)

// inProgressMarkers are the paths git writes while an operation stops for the
// user, and removes when it ends.
var inProgressMarkers = map[string]string{
	"MERGE_HEAD":       "a merge",
	"rebase-merge":     "a rebase",
	"rebase-apply":     "a rebase",
	"CHERRY_PICK_HEAD": "a cherry-pick",
	"REVERT_HEAD":      "a revert",
}

// errNoGit marks a root git does not track, where no branch scopes the sweep.
var errNoGit = errors.New("not inside a git work tree")

// openRoot opens the work tree that holds root.
func openRoot(root string) (*gitread.Repo, error) {
	repo, err := gitread.OpenWorkTree(root)
	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, errNoGit
	}
	return repo, nil
}

// operationInProgress names the git operation stopped in the work tree at
// root, or answers "".
func operationInProgress(root string) (string, error) {
	repo, err := openRoot(root)
	if err != nil {
		return "", err
	}
	for marker, name := range inProgressMarkers {
		if _, err := os.Stat(repo.Path(marker)); err == nil {
			return name, nil
		}
	}
	return "", nil
}

// defaultBranch answers the commit and the ref of the remote's default branch.
func defaultBranch(repo *gitread.Repo) (gitread.OID, string, error) {
	refs := []string{"refs/remotes/origin/main", "refs/remotes/origin/master", "refs/heads/main", "refs/heads/master"}
	if target, ok := repo.SymbolicRef("refs/remotes/origin/HEAD"); ok {
		refs = append([]string{target}, refs...)
	}
	for _, ref := range refs {
		if oid, err := repo.Resolve(ref); err == nil {
			return oid, ref, nil
		}
	}
	return gitread.OID{}, "", errors.New("no default branch: origin/HEAD, main and master are all missing")
}

// changedFiles answers every file under root that differs from the merge base
// with the default branch: committed on the branch, staged, unstaged or new.
func changedFiles(root string) (set.Set[string], error) {
	repo, err := openRoot(root)
	if err != nil {
		return set.New[string](), err
	}
	branch, ref, err := defaultBranch(repo)
	if err != nil {
		return set.New[string](), err
	}
	head, err := repo.Head()
	if err != nil {
		return set.New[string](), fmt.Errorf("no merge base with %s: %w", ref, err)
	}
	base, ok, err := repo.MergeBase(head, branch)
	if err != nil {
		return set.New[string](), fmt.Errorf("no merge base with %s: %w", ref, err)
	}
	if !ok {
		return set.New[string](), fmt.Errorf("no merge base with %s", ref)
	}
	names, err := repo.ChangedNames(base)
	if err != nil {
		return set.New[string](), err
	}
	out := set.New[string]()
	for _, name := range names {
		out.Add(filepath.Join(repo.WorkTree(), filepath.FromSlash(name)))
	}
	return out, nil
}
