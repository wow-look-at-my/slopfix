// Package gitmod derives the directories a walk must not judge: this
// repository's registered submodules, which carry their own CI.
//
// The list is DERIVED, never declared. No flag, input or environment variable
// adds a path to it. That is the point: an exclusion a caller writes is an
// exclusion a caller can widen to everything, and a check somebody can switch
// off enforces nothing.
package gitmod

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// gitlinkMode is the index mode git gives a submodule entry.
const gitlinkMode = "160000"

// Skip returns the submodule directories that contain target, each spelled the
// way Resolved spells a path. A caller tests a directory of its own against
// this set, and both spellings have to agree.
func Skip(target string) (set.Set[string], error) {
	empty := set.New[string]()

	root, ok := topLevel(target)
	if !ok {
		return empty, nil
	}
	root = Resolved(root)
	declared, ok := declaredPaths(root)
	if !ok {
		return empty, nil
	}

	out := set.New[string]()
	for _, path := range declared {
		if err := verify(root, path); err != nil {
			return empty, err
		}
		out.Add(filepath.Join(root, filepath.FromSlash(path)))
	}
	return out, nil
}

// Resolved spells a path the thing way Skip's entries are spelled:
// absolute, with every symlink on the way followed.
//
// A directory reached through a symlink has names, and a set holding a single
// answers no to the other. A walk asking about /var/folders/x/vendored misses
// the entry for /private/var/folders/x/vendored and judges the submodule's
// files as though they were this repository's own. Every temporary directory
// on macOS is such a symlink, and a work tree under a single is ordinary.
//
// A path that does not exist keeps its absolute spelling, because a caller
// asking about it wants an answer rather than an error.
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
func verify(root, path string) error {
	if path == "" || path == "." || filepath.IsAbs(path) ||
		path != filepath.ToSlash(filepath.Clean(path)) || strings.HasPrefix(path, "../") {
		return fmt.Errorf(".gitmodules names %q, which is not a path inside this repository", path)
	}
	staged, ok := git(root, "ls-files", "--stage", "--", path)
	if !ok {
		return fmt.Errorf("cannot read the index entry for the submodule %q", path)
	}
	if !strings.HasPrefix(strings.TrimSpace(staged), gitlinkMode+" ") {
		return fmt.Errorf(".gitmodules names %q as a submodule, but the index has no gitlink there."+
			" A submodule carries its own CI, so its files are the only ones this skips;"+
			" an entry that is not one would exempt this repository's own source", path)
	}
	return nil
}

// declaredPaths reads the path of every submodule .gitmodules registers.
func declaredPaths(root string) ([]string, bool) {
	if _, err := os.Stat(filepath.Join(root, ".gitmodules")); err != nil {
		return nil, false
	}
	out, ok := git(root, "config", "--file", ".gitmodules", "--get-regexp", `^submodule\..*\.path$`)
	if !ok {
		return nil, false
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		_, value, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found {
			continue
		}
		if value = strings.TrimRight(strings.TrimSpace(value), "/"); value != "" {
			paths = append(paths, value)
		}
	}
	return paths, true
}

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

func git(dir string, args ...string) (string, bool) {
	command := exec.Command("git", args...)
	command.Dir = dir
	out, err := command.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}
