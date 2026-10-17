package gitread

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// pattern is one gitignore or gitattributes pattern, parsed once.
type pattern struct {
	glob string
	// base is the directory the pattern is relative to, "" for the root.
	base    string
	negate  bool
	dirOnly bool
	// basename matches the last path element at any depth below base.
	basename bool
	// literal and suffix give a pattern with no wildcard a fast path.
	literal bool
	suffix  string
}

func hasWildcard(s string) bool { return strings.ContainsAny(s, `*?[\`) }

// newPattern parses a pattern, with the negation and trailing slash already cut.
func newPattern(glob, base string) pattern {
	p := pattern{base: base}
	if !strings.Contains(glob, "/") {
		p.basename = true
	}
	glob = strings.TrimPrefix(glob, "/")
	p.glob = glob
	p.literal = !hasWildcard(glob)
	if strings.HasPrefix(glob, "*") && !hasWildcard(glob[1:]) {
		p.suffix = glob[1:]
	}
	return p
}

// matches reports whether the pattern matches rel, a path relative to the
// repository root, whose last element is name.
func (p *pattern) matches(rel, name string, isDir, fold bool) bool {
	if p.dirOnly && !isDir {
		return false
	}
	if p.basename {
		if p.base != "" && !strings.HasPrefix(rel, p.base+"/") {
			return false
		}
		switch {
		case p.literal && !fold:
			return name == p.glob
		case p.literal:
			return strings.EqualFold(name, p.glob)
		case p.suffix != "" && !fold:
			return strings.HasSuffix(name, p.suffix)
		}
		return wildmatch(p.glob, name, false, fold)
	}
	sub := rel
	if p.base != "" {
		rest, ok := strings.CutPrefix(rel, p.base+"/")
		if !ok {
			return false
		}
		sub = rest
	}
	if p.literal && !fold {
		return sub == p.glob
	}
	return wildmatch(p.glob, sub, true, fold)
}

// ruleFile is one parsed rule file, with the stamp it was read at.
type ruleFile[T any] struct {
	stamp fileStamp
	rules []T
}

// ruleCache holds every rule file a repository read. An entry is used while
// the file on disk keeps its stamp.
type ruleCache struct {
	ignores sync.Map
	attrs   sync.Map
}

// load answers the rules of a file, read again when its stamp moved. A
// symlink is not followed, as git does not follow one in the work tree.
func load[T any](cache *sync.Map, path string, parse func([]byte) []T) []T {
	stamp := stampOf(path)
	if v, ok := cache.Load(path); ok {
		if f := v.(*ruleFile[T]); f.stamp == stamp {
			return f.rules
		}
	}
	var rules []T
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
		if data, err := os.ReadFile(path); err == nil {
			rules = parse(data)
		}
	}
	cache.Store(path, &ruleFile[T]{stamp: stamp, rules: rules})
	return rules
}

// dirOf answers the directory part of a slash path, "" at the root.
func dirOf(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return ""
}

func baseOf(rel string) string { return rel[strings.LastIndexByte(rel, '/')+1:] }

// globalFile answers a path config names, or git's default under the XDG
// config directory.
func (r *Repo) globalFile(key, name string) string {
	if path := r.Config(key); path != "" {
		return expandHome(path)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "git", name)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "git", name)
	}
	return ""
}
