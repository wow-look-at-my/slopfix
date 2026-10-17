// Package gitmod derives the directories a walk must not judge: this
// repository's registered submodules, which carry their own CI.
//
// The list is DERIVED, never declared. No flag, input or environment variable
// adds a path to it. That is the point: an exclusion a caller writes is an
// exclusion a caller can widen to everything. A check somebody can switch off
// enforces nothing.
package gitmod

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitread"
)

// gitlinkMode is the index mode git gives a submodule entry.
const gitlinkMode = "160000"

// Skip returns the submodule directories that contain target, each spelled the
// way Resolved spells a path. A caller tests a directory of its own against
// this set, and both spellings have to agree.
func Skip(target string) (set.Set[string], error) {
	empty := set.New[string]()

	repo, err := gitread.OpenWorkTree(target)
	if err != nil || repo == nil {
		return empty, nil
	}
	root := Resolved(repo.WorkTree())
	declared, ok := declaredPaths(root)
	if !ok {
		return empty, nil
	}

	out := set.New[string]()
	for _, path := range declared {
		if err := verify(repo, path); err != nil {
			return empty, err
		}
		out.Add(filepath.Join(root, filepath.FromSlash(path)))
	}
	return out, nil
}

// Resolved spells a path the thing way Skip's entries are spelled:
// absolute, with every symlink on the way followed.
func Resolved(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	real, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return absolute
	}
	return real
}

// verify requires the declared path to be a gitlink in the index. A directory
// holding this repository's own source cannot become a gitlink without
// replacing that source with a commit pointer.
func verify(repo *gitread.Repo, path string) error {
	if path == "" || path == "." || filepath.IsAbs(path) ||
		path != filepath.ToSlash(filepath.Clean(path)) || strings.HasPrefix(path, "../") {
		return fmt.Errorf(".gitmodules names %q, which is not a path inside this repository", path)
	}
	idx, err := repo.Index()
	if err != nil {
		return fmt.Errorf("cannot read the index entry for the submodule %q", path)
	}
	entry, ok := idx.Tracked()[path]
	if !ok || entry.Mode != gitlinkMode {
		return fmt.Errorf(".gitmodules names %q as a submodule, but the index has no gitlink there."+
			" A submodule carries its own CI, so its files are the only ones this skips;"+
			" an entry that is not one would exempt this repository's own source", path)
	}
	return nil
}

// declaredPaths reads the path of every submodule .gitmodules registers.
func declaredPaths(root string) ([]string, bool) {
	file, err := os.Open(filepath.Join(root, ".gitmodules"))
	if err != nil {
		return nil, false
	}
	defer file.Close()
	var paths []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "path" {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.TrimRight(value, "/")
		if value != "" {
			paths = append(paths, value)
		}
	}
	return paths, true
}

// TopLevel resolves the work tree holding target, absolute and free of symlinks.
func TopLevel(target string) (string, bool) { return topLevel(target) }

// topLevel resolves the work tree holding target.
func topLevel(target string) (string, bool) {
	dir := target
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	out, ok := git(dir, "rev-parse", "--show-toplevel")
	if !ok {
		return "", false
	}
	root, err := Resolve(strings.TrimSpace(out))
	if err != nil {
		return "", false
	}
	return root, true
}

// Resolve returns path absolute and free of symlinks, so both sides of a
// comparison spell a directory alike.
func Resolve(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

// Command is a git read in the work tree slopfix judges. It trusts that tree
// whatever user owns it. A CI container checks out as another user.
func Command(dir string, args ...string) *exec.Cmd {
	return CommandContext(context.Background(), dir, args...)
}

// CommandContext is Command with a deadline.
func CommandContext(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "safe.directory=*"}, args...)...)
	cmd.Dir = dir
	return cmd
}

// Unexpected answers why a git read failed, or "" when the cause is only that
// the path is outside a work tree. A caller reports a reason it gets back,
// because the read it lost turns a skip off.
func Unexpected(err error) string {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return err.Error()
	}
	reason := strings.TrimSpace(string(exit.Stderr))
	if strings.Contains(reason, "not a git repository") {
		return ""
	}
	return reason
}

func git(dir string, args ...string) (string, bool) {
	out, err := Command(dir, args...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}
