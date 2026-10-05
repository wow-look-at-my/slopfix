package slopfix

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/wow-look-at-my/go-containers/set"
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
	top := workTreeOf(filepath.Dir(abs))
	if top == "" {
		return false
	}
	files, _ := embeddedByTop.LoadOrStore(top, sync.OnceValue(func() set.Set[string] { return embeddedIn(top) }))
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		resolved = abs
	}
	return files.(func() set.Set[string])().Contains(resolved)
}

// workTreeOf answers the root of the git work tree that holds dir, or "".
func workTreeOf(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// embeddedIn answers the absolute path of each file a Rust source under top
// includes, with every symlink resolved.
func embeddedIn(top string) set.Set[string] {
	files := set.New[string]()
	out, err := exec.Command("git", "-C", top, "grep", "-I", "-n", "-z", "-E", `include_(str|bytes)!`, "--", "*.rs").Output()
	if err != nil {
		return files
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		name, rest, ok := strings.Cut(line, "\x00")
		if !ok {
			continue
		}
		for _, m := range includeMacro.FindAllStringSubmatch(rest, -1) {
			target := filepath.Join(top, filepath.Dir(name), m[1])
			if resolved, err := filepath.EvalSymlinks(target); err == nil {
				target = resolved
			}
			files.Add(target)
		}
	}
	return files
}
