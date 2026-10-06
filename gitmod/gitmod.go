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
	"fmt"
	"os"
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
