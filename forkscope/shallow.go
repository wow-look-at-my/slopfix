// A shallow clone of a fork reaches its parent's history in steps, never whole.

package forkscope

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/wow-look-at-my/slopfix/gitread"
)

// Unborn reports whether path is in a git work tree whose index and HEAD hold
// nothing at that path. The fork wrote all of such a file, so its scope needs
// no base and no network. It answers false when it cannot tell.
func Unborn(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	repo, err := gitread.OpenWorkTree(ExistingDir(filepath.Dir(abs)))
	if err != nil || repo == nil {
		return false
	}
	rel, ok := repo.Rel(abs)
	if !ok || rel == "." {
		return false
	}
	idx, err := repo.Index()
	if err != nil {
		return false
	}
	if _, tracked := idx.Tracked()[rel]; tracked {
		return false
	}
	head, err := repo.Resolve("HEAD")
	if err != nil {
		return false
	}
	if head, err = repo.PeelCommit(head); err != nil {
		return false
	}
	_, inHead, err := repo.TreeEntryAt(head, rel)
	return err == nil && !inHead
}

// fetchParent fetches the parent's branch into top and answers its commit.
// Only commits and trees cross the wire. A shallow clone takes the tip alone:
// the parent knows none of the fork's own commits. A full fetch would carry
// the parent's whole history. parentBase deepens it as far as it must.
func fetchParent(top, parentURL, branch string) (string, error) {
	args := []string{"fetch", "--quiet", "--filter=blob:none", "--no-tags"}
	if shallow(top) {
		args = append(args, "--depth=1")
	}
	if _, err := gitIn(top, append(args, parentURL, "refs/heads/"+branch)...); err != nil {
		return "", fmt.Errorf("fork scope: fetch the parent's %s from %s: %w", branch, parentURL, err)
	}
	commit, _, err := resolve(top, "FETCH_HEAD")
	if err != nil {
		return "", fmt.Errorf("fork scope: resolve the parent's %s: %w", branch, err)
	}
	return commit.String(), nil
}

// parentBase answers the merge base of HEAD and the parent's tip. A shallow
// clone gets more history from both remotes in doubling steps, until the base
// shows. Each side then holds about as many generations as the step. The
// download grows with the distance to the fork point, not with the history.
// It stops when a step brings in nothing.
func parentBase(top string, rec record) (string, error) {
	for depth := 2; ; depth *= 2 {
		base, err := mergeBase(top, "HEAD", rec.Parent)
		if err == nil || !shallow(top) {
			return base, err
		}
		before := shallowFile(top)
		if _, err := gitIn(top, "fetch", "--quiet", "--filter=blob:none", "--no-tags",
			"--depth="+strconv.Itoa(depth), rec.ParentURL, "refs/heads/"+rec.Branch); err != nil {
			return "", fmt.Errorf("fork scope: deepen the parent's %s from %s: %w", rec.Branch, rec.ParentURL, err)
		}
		if _, err := gitIn(top, "fetch", "--quiet", "--filter=blob:none", "--no-tags",
			"--deepen="+strconv.Itoa(depth/2), "origin"); err != nil {
			return "", fmt.Errorf("fork scope: deepen the shallow clone from origin: %w", err)
		}
		if shallowFile(top) == before {
			return mergeBase(top, "HEAD", rec.Parent)
		}
	}
}

// shallow reports whether the work tree at top is a shallow clone.
func shallow(top string) bool {
	repo, err := openTop(top)
	return err == nil && repo.IsShallow()
}

// shallowFile answers the commits the shallow file of top names, as git wrote them.
func shallowFile(top string) string {
	repo, err := openTop(top)
	if err != nil {
		return ""
	}
	data, _ := os.ReadFile(filepath.Join(repo.CommonDir(), "shallow"))
	return string(data)
}
