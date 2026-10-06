package gitread

import (
	"os"
	"path/filepath"
	"strings"
)

func relPath(root, path string) (string, error) {
	return filepath.Rel(root, path)
}

// ignoreLine is one parsed gitignore line.
type ignoreLine struct {
	pattern  string
	negate   bool
	dirOnly  bool
	relative string // directory the pattern is relative to, "" for the root
}

// Ignore is the parsed ignore rules of one repository.
type Ignore struct {
	lines []ignoreLine
}

// Ignore reads the repository's ignore rules, cached in the caller.
func (r *Repo) Ignore() *Ignore {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ignore != nil {
		return r.ignore
	}
	out := &Ignore{}
	if r.workTree == "" {
		r.ignore = out
		return out
	}
	exclude := filepath.Join(r.commonDir, "info", "exclude")
	out.lines = append(out.lines, readIgnoreFile(exclude, "")...)
	out.lines = append(out.lines, readIgnoreTree(r.workTree, "")...)
	r.ignore = out
	return out
}

// readIgnoreTree reads every .gitignore under a directory, breadth first.
func readIgnoreTree(root, prefix string) []ignoreLine {
	dir := filepath.Join(root, filepath.FromSlash(prefix))
	out := readIgnoreFile(filepath.Join(dir, ".gitignore"), prefix)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == ".git" {
			continue
		}
		child := e.Name()
		if prefix != "" {
			child = prefix + "/" + e.Name()
		}
		out = append(out, readIgnoreTree(root, child)...)
	}
	return out
}

// readIgnoreFile parses one ignore file, whose patterns are relative to dir.
func readIgnoreFile(path, dir string) []ignoreLine {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []ignoreLine
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimRight(line, " ")
		negate := strings.HasPrefix(line, "!")
		if negate {
			line = line[1:]
		}
		if strings.HasPrefix(line, `\#`) || strings.HasPrefix(line, `\!`) {
			line = line[1:]
		}
		if line == "" {
			continue
		}
		dirOnly := strings.HasSuffix(line, "/")
		line = strings.TrimSuffix(line, "/")
		out = append(out, ignoreLine{pattern: line, negate: negate, dirOnly: dirOnly, relative: dir})
	}
	return out
}

// Ignored reports whether a repo-relative path is ignored.
func (i *Ignore) Ignored(rel string, isDir bool) bool {
	ignored := false
	for _, line := range i.lines {
		if line.dirOnly && !isDir {
			continue
		}
		subject := rel
		if line.relative != "" {
			rest, ok := strings.CutPrefix(rel, line.relative+"/")
			if !ok {
				continue
			}
			subject = rest
		}
		if globMatch(line.pattern, subject) {
			ignored = !line.negate
		}
	}
	return ignored
}
