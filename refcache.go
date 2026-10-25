// refsFile is the file under a repository's .git that caches ref answers.

package slopfix

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/slopfix/gitmod"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// refsFile is the file under a repository's .git that caches ref answers.
const refsFile = "slopfix-refs.json"

// commonGitDir answers the git directory that every work tree of the
// repository at root shares, where the ref cache lives. A .git directory is
// that directory. In a linked work tree .git is a file that points into the
// main repository, so git names the directory.
func commonGitDir(root string) (string, error) {
	dotGit := filepath.Join(root, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return dotGit, nil
	}
	out, err := gitmod.Command(root, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir in %s: %w", root, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// recordedRefs answers from the recorded cache, and asks next for every
// repository or ref the cache lacks.
type recordedRefs struct {
	table map[string]string
	next  workflow.Refs
}

func (r recordedRefs) DefaultBranch(repo string) (string, error) {
	if branch, ok := r.table[repo]; ok {
		return branch, nil
	}
	return r.next.DefaultBranch(repo)
}

func (r recordedRefs) Kind(repo, ref string) (workflow.RefKind, error) {
	switch r.table[repo+"@"+ref] {
	case "branch":
		return workflow.RefBranch, nil
	case "tag":
		return workflow.RefTag, nil
	}
	return r.next.Kind(repo, ref)
}

// refsAt answers the lookups for the repository at root: the ones recorded
// under it, and next for the rest. With nothing recorded it answers next.
func refsAt(root string, next workflow.Refs) (workflow.Refs, error) {
	dir, err := commonGitDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return next, nil
	}
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, refsFile))
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
