// scope.go decides which files a build sweep may write.
//
// A build runs the sweep on every tree it touches, and a rewrite there lands
// in the author's working copy. So the sweep writes only what the branch
// already changed, and nothing while git is part way through a merge or a
// rebase: a rewrite then mixes into the conflict resolution.
package commentfix

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
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

// git runs a git command in dir and answers its trimmed output.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exit.Stderr)))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// operationInProgress names the git operation stopped in the work tree at
// root, or answers "".
func operationInProgress(root string) (string, error) {
	for marker, name := range inProgressMarkers {
		path, err := git(root, "rev-parse", "--git-path", marker)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if _, err := os.Stat(path); err == nil {
			return name, nil
		}
	}
	return "", nil
}

// defaultBranch answers the ref of the remote's default branch.
func defaultBranch(root string) (string, error) {
	if ref, err := git(root, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil && ref != "" {
		return ref, nil
	}
	for _, ref := range []string{"refs/remotes/origin/main", "refs/remotes/origin/master", "refs/heads/main", "refs/heads/master"} {
		if _, err := git(root, "rev-parse", "--verify", "--quiet", ref); err == nil {
			return ref, nil
		}
	}
	return "", errors.New("no default branch: origin/HEAD, main and master are all missing")
}

// changedFiles answers every file under root that differs from the merge base
// with the default branch: committed on the branch, staged, unstaged or new.
func changedFiles(root string) (set.Set[string], error) {
	top, err := git(root, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, errNoGit
	}
	branch, err := defaultBranch(root)
	if err != nil {
		return nil, err
	}
	base, err := git(root, "merge-base", "HEAD", branch)
	if err != nil {
		return nil, fmt.Errorf("no merge base with %s: %w", branch, err)
	}
	diffed, err := git(top, "diff", "--name-only", "--no-renames", base, "--")
	if err != nil {
		return nil, err
	}
	untracked, err := git(top, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	out := set.New[string]()
	for _, name := range append(strings.Split(diffed, "\n"), strings.Split(untracked, "\n")...) {
		if name == "" {
			continue
		}
		out.Add(filepath.Clean(filepath.Join(top, name)))
	}
	return out, nil
}
