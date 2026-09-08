package noworkloss

import (
	"fmt"
	"os"
	"strings"
)

// protectedRefPrefix names a ref that holds content nothing else does.
const protectedRefPrefix = "refs/no-work-loss/"

// preserveResult is what a successful commit produced. ref names the branch
// the commit landed on.
type preserveResult struct {
	ref     string
	commit  string
	pushed  bool
	pushErr string
}

// preserveAtRiskPaths commits the paths a destructive command would destroy.
// It never touches the user's index, and ok is false when the commit failed.
func preserveAtRiskPaths(root string, paths []string) (res *preserveResult, ok bool) {
	if root == "" || len(paths) == 0 {
		return nil, false
	}

	tmp, err := os.CreateTemp("", "no-work-loss-index-*")
	if err != nil {
		return nil, false
	}
	tmpIndex := tmp.Name()
	tmp.Close()
	// An empty FILE is not an empty index: git refuses it. A missing
	// GIT_INDEX_FILE starts from a genuinely empty index.
	os.Remove(tmpIndex)
	defer os.Remove(tmpIndex)
	env := []string{"GIT_INDEX_FILE=" + tmpIndex}

	// read-tree HEAD seeds the temp index with the last committed tree. A
	// repository with no commits has no HEAD, so the index starts empty.
	hasHead := true
	if _, _, err := runGitEnvTimeout(root, gitTimeout, env, "read-tree", "HEAD"); err != nil {
		hasHead = false
	}

	headTree := ""
	if hasHead {
		out, _, err := runGit(root, "rev-parse", "HEAD^{tree}")
		if err == nil {
			headTree = strings.TrimSpace(out)
		}
	}

	// A staged version is a SEPARATE state that lives only in the index, so it
	// is captured here and the working tree lands on top of it.
	var trees []string
	if t := stagedTree(root, env, paths, hasHead); t != "" && t != headTree {
		trees = append(trees, t)
	}

	// --force: an ignored file (clean -fdx) is exactly the case `git add`
	// otherwise refuses to stage, and a tracked or plain untracked path is
	// unaffected by the flag.
	addArgs := append([]string{"add", "--force", "--"}, paths...)
	if _, _, err := runGitEnvTimeout(root, gitTimeout, env, addArgs...); err != nil {
		return nil, false
	}

	treeOut, _, err := runGitEnvTimeout(root, gitTimeout, env, "write-tree")
	if err != nil {
		return nil, false
	}
	tree := strings.TrimSpace(treeOut)
	if tree == "" {
		return nil, false
	}
	if len(trees) == 0 || trees[len(trees)-1] != tree {
		trees = append(trees, tree)
	}

	commit := ""
	parent := ""
	if hasHead {
		parent = "HEAD"
	}
	for i, t := range trees {
		commitArgs := []string{"commit-tree", t, "-m", preserveMessage(paths, i == len(trees)-1)}
		if parent != "" {
			commitArgs = append(commitArgs, "-p", parent)
		}
		commitOut, _, err := runGit(root, commitArgs...)
		if err != nil {
			return nil, false
		}
		commit = strings.TrimSpace(commitOut)
		if commit == "" {
			return nil, false
		}
		parent = commit
	}

	// The commit lands on the CURRENT BRANCH, where the log, the diff and the next push all show it.
	if _, _, err := runGit(root, "update-ref", "HEAD", commit); err != nil {
		// The commit object exists but nothing names it, so git gc can reap
		// it. That is not durable preservation, so this must not read as such.
		return nil, false
	}
	// The branch moved under the real index, which would then report a STAGED
	// REVERT. Only the preserved paths are refreshed.
	resetArgs := append([]string{"reset", "-q", commit, "--"}, paths...)
	runGit(root, resetArgs...)

	res = &preserveResult{ref: branchName(root), commit: commit}
	if _, stderr, err := runGitEnvTimeout(root, preservePushTimeout, nil, "push", "origin", "HEAD"); err != nil {
		res.pushErr = strings.TrimSpace(stderr)
		if res.pushErr == "" {
			res.pushErr = err.Error()
		}
	} else {
		res.pushed = true
	}
	return res, true
}

// stagedTree builds a tree holding HEAD's content except at the at-risk paths, which take the USER'S index.
func stagedTree(root string, env, paths []string, hasHead bool) string {
	// Ask up front, in a single call, which at-risk paths have anything staged,
	// so the ordinary tree pays nothing. A repository with no HEAD has no tree
	// to differ from.
	if !hasHead {
		return ""
	}
	diffArgs := append([]string{"diff", "--cached", "--name-only", "-z", "--"}, paths...)
	diff, _, err := runGit(root, diffArgs...)
	if err != nil {
		return ""
	}
	var changed []string
	for _, p := range strings.Split(diff, "\x00") {
		if p != "" {
			changed = append(changed, p)
		}
	}
	if len(changed) == 0 {
		return ""
	}

	args := append([]string{"ls-files", "--stage", "-z", "--"}, changed...)
	out, _, err := runGit(root, args...)
	if err != nil {
		return ""
	}
	staged := 0
	for _, rec := range strings.Split(out, "\x00") {
		meta, path, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 || fields[2] != "0" {
			// A non-default stage is an unresolved merge conflict. Its entries do
			// not make a tree, and a conflicted path is not a state a commit
			// can hold, so it is left to the working-tree pass below.
			continue
		}
		info := fields[0] + "," + fields[1] + "," + path
		if _, _, err := runGitEnvTimeout(root, gitTimeout, env, "update-index", "--add", "--cacheinfo", info); err != nil {
			return ""
		}
		staged++
	}
	if staged == 0 {
		return ""
	}
	tree, _, err := runGitEnvTimeout(root, gitTimeout, env, "write-tree")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(tree)
}

func preserveMessage(paths []string, working bool) string {
	what := "the staged version of"
	if working {
		what = "the working-tree version of"
	}
	return fmt.Sprintf("no-work-loss: preserved %s %d path(s) before a destructive command\n\n%s",
		what, len(paths), strings.Join(paths, "\n"))
}

// branchName is the branch HEAD points at, for the notice. A detached HEAD
// has no name, and saying so is better than printing an empty name.
func branchName(root string) string {
	out, _, err := runGit(root, "rev-parse", "--abbrev-ref", "HEAD")
	name := strings.TrimSpace(out)
	if err != nil || name == "" || name == "HEAD" {
		return "a detached HEAD"
	}
	return name
}

// notice reports the preservation. The commit is on the branch, so it is
// visible in the log without this -- but a commit the session did not write
// itself must still be announced. label is the finding's own name for the
// command that would have destroyed the content; summary is the same
// tracked/untracked/ignored breakdown a denial would have shown.
func (r *preserveResult) notice(label, summary string) string {
	if r.pushed {
		return fmt.Sprintf("preserved: %s would have lost %s, so it was committed to %s (%s) and pushed before being allowed to proceed.",
			label, summary, r.ref, shortSHA(r.commit))
	}
	return fmt.Sprintf("preserved: %s would have lost %s, so it was committed to %s (%s) before being allowed to proceed. The push failed (%s) -- the commit is on your branch; push it when you can.",
		label, summary, r.ref, shortSHA(r.commit), r.pushErr)
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// isProtectedRef reports a preservation ref. Deleting it is always refused,
// because it IS the somewhere else.
func isProtectedRef(ref string) bool {
	return ref != "" && strings.HasPrefix(ref, protectedRefPrefix)
}

// protectedRefFinding is the unconditional denial for a command that names a
// preservation ref. It skips the reachability question entirely: a ref under
// protectedRefPrefix is by definition the only place its content survives, so
// asking "does it exist somewhere else" would always answer no in the way
// that matters and yes in the way that does not (the ref itself, trivially).
func protectedRefFinding(dir, label string) *finding {
	return &finding{
		label: label, always: true, dir: dir,
		reason: "blocked: " + label + " names a ref this hook created to preserve content before a destructive command; it is the only copy and must not be deleted or overwritten.",
		rewrite: "git fetch origin " + protectedRefPrefix + "*:" + protectedRefPrefix + "*   " +
			"# recover it first if you need to, then work with a different ref",
	}
}
