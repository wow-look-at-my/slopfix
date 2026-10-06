package slopfix

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitread"
)

// includeMacro matches a Rust include_str! or include_bytes! of a literal path.
var includeMacro = regexp.MustCompile(`include_(?:str|bytes)!\(\s*"([^"]+)"\s*\)`)

// embeddedByTop caches, for each work tree root, the files its code embeds.
var embeddedByTop sync.Map

// Embedded reports whether code in the repository of path embeds the file at
// path byte for byte. Such a file is program input, not prose. A reworded
// prompt template changes what the program does. A copy the build checks
// against it goes stale. No rule reads it, and no repair writes it.
func Embedded(path string) bool {
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	repo, err := gitread.OpenWorkTree(filepath.Dir(abs))
	if err != nil || repo == nil {
		return false
	}
	files, _ := embeddedByTop.LoadOrStore(repo.WorkTree(), sync.OnceValue(func() set.Set[string] { return embeddedIn(repo) }))
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		resolved = abs
	}
	return files.(func() set.Set[string])().Contains(resolved)
}

// embeddedIn answers the absolute path of each file a Rust source in the work
// tree includes, with every symlink resolved. It starts no git process.
func embeddedIn(repo *gitread.Repo) set.Set[string] {
	files := set.New[string]()
	names, err := repo.WorkFiles()
	if err != nil {
		return files
	}
	top := repo.WorkTree()
	for _, name := range names {
		if !strings.HasSuffix(name, ".rs") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(top, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		for _, m := range includeMacro.FindAllStringSubmatch(string(src), -1) {
			target := filepath.Join(top, filepath.Dir(filepath.FromSlash(name)), m[1])
			if resolved, err := filepath.EvalSymlinks(target); err == nil {
				target = resolved
			}
			files.Add(target)
		}
	}
	return files
}
