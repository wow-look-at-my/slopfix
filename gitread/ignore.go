package gitread

import (
	"path/filepath"
	"strings"
	"sync"
)

// Ignore answers whether git ignores a path, as git check-ignore does for an
// untracked path. Its rules come from each .gitignore on the way down,
// info/exclude, and core.excludesFile. It reads each directory's rules once.
type Ignore struct {
	repo *Repo
	fold bool
	base [][]pattern

	mu   sync.Mutex
	dirs map[string][]pattern
	// excluded remembers each directory already judged.
	excluded map[string]bool
}

// Ignore answers a matcher that reads the rule files as they stand now.
func (r *Repo) Ignore() *Ignore {
	ig := &Ignore{repo: r, fold: r.ConfigBool("core.ignorecase", false), dirs: map[string][]pattern{}, excluded: map[string]bool{}}
	if global := r.globalFile("core.excludesfile", "ignore"); global != "" {
		ig.base = append(ig.base, load(&r.rules.ignores, global, parseIgnore("")))
	}
	ig.base = append(ig.base, load(&r.rules.ignores, filepath.Join(r.commonDir, "info", "exclude"), parseIgnore("")))
	return ig
}

func parseIgnore(base string) func([]byte) []pattern {
	return func(data []byte) []pattern {
		var out []pattern
		for _, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimSuffix(raw, "\r")
			if line == "" || line[0] == '#' {
				continue
			}
			line = trimTrailingSpace(line)
			negate := false
			if line[0] == '!' {
				negate = true
				line = line[1:]
			} else if line[0] == '\\' && len(line) > 1 && (line[1] == '#' || line[1] == '!') {
				line = line[1:]
			}
			dirOnly := strings.HasSuffix(line, "/")
			line = strings.TrimSuffix(line, "/")
			if line == "" {
				continue
			}
			p := newPattern(line, base)
			p.negate, p.dirOnly = negate, dirOnly
			out = append(out, p)
		}
		return out
	}
}

// trimTrailingSpace drops trailing spaces that a backslash does not escape.
func trimTrailingSpace(line string) string {
	end := len(line)
	for end > 0 && line[end-1] == ' ' {
		if end >= 2 && line[end-2] == '\\' {
			return line[:end-2] + " "
		}
		end--
	}
	return line[:end]
}

// rulesIn answers the patterns of dir's .gitignore.
func (ig *Ignore) rulesIn(dir string) []pattern {
	ig.mu.Lock()
	rules, ok := ig.dirs[dir]
	ig.mu.Unlock()
	if ok {
		return rules
	}
	rules = load(&ig.repo.rules.ignores, filepath.Join(ig.repo.workTree, filepath.FromSlash(dir), ".gitignore"), parseIgnore(dir))
	ig.mu.Lock()
	ig.dirs[dir] = rules
	ig.mu.Unlock()
	return rules
}

// Ignored reports whether a path relative to the work tree is ignored. A path
// under an ignored directory is ignored, since git cannot include it again.
func (ig *Ignore) Ignored(rel string, isDir bool) bool {
	if ig.repo.workTree == "" || rel == "" {
		return false
	}
	for dir := dirOf(rel); dir != ""; dir = dirOf(dir) {
		if ig.dirExcluded(dir) {
			return true
		}
	}
	return ig.own(rel, isDir)
}

// dirExcluded judges a directory, with every ancestor of it judged first.
func (ig *Ignore) dirExcluded(dir string) bool {
	ig.mu.Lock()
	v, ok := ig.excluded[dir]
	ig.mu.Unlock()
	if ok {
		return v
	}
	v = false
	if parent := dirOf(dir); parent != "" && ig.dirExcluded(parent) {
		v = true
	} else {
		v = ig.own(dir, true)
	}
	ig.mu.Lock()
	ig.excluded[dir] = v
	ig.mu.Unlock()
	return v
}

// own judges a path by the rules alone. The nearest .gitignore wins, and in
// one file the last matching line wins.
func (ig *Ignore) own(rel string, isDir bool) bool {
	name := baseOf(rel)
	dir := dirOf(rel)
	for {
		if decided, ignored := ig.decide(ig.rulesIn(dir), rel, name, isDir); decided {
			return ignored
		}
		if dir == "" {
			break
		}
		dir = dirOf(dir)
	}
	for i := len(ig.base) - 1; i >= 0; i-- {
		if decided, ignored := ig.decide(ig.base[i], rel, name, isDir); decided {
			return ignored
		}
	}
	return false
}

func (ig *Ignore) decide(rules []pattern, rel, name string, isDir bool) (bool, bool) {
	for i := len(rules) - 1; i >= 0; i-- {
		if rules[i].matches(rel, name, isDir, ig.fold) {
			return true, !rules[i].negate
		}
	}
	return false, false
}
