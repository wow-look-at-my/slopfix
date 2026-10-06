package gitread

import (
	"os"
	"path/filepath"
	"strings"
)

// Attributes is the parsed gitattributes rules of one repository.
type Attributes struct {
	lines []attrLine
}

type attrLine struct {
	pattern  string
	relative string
	attrs    map[string]string
}

// Attr answers one attribute's value for a repo-relative path: "set", "unset",
// "unspecified", or the text after '='.
func (r *Repo) Attr(rel, name string) string {
	attrs := r.attributes()
	value := "unspecified"
	for _, line := range attrs.lines {
		subject := rel
		if line.relative != "" {
			rest, ok := strings.CutPrefix(rel, line.relative+"/")
			if !ok {
				continue
			}
			subject = rest
		}
		if !globMatch(line.pattern, subject) {
			continue
		}
		if v, ok := line.attrs[name]; ok {
			value = v
		}
	}
	return value
}

// AttributeSet reports a value that turns an attribute on.
func AttributeSet(value string) bool {
	switch value {
	case "unspecified", "unset", "false":
		return false
	}
	return true
}

// attributes answers the repository's gitattributes rules, cached.
func (r *Repo) attributes() *Attributes {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.attrs != nil {
		return r.attrs
	}
	out := &Attributes{}
	if r.workTree == "" {
		r.attrs = out
		return out
	}
	// Lowest precedence first: the info file is not here; it applies last.
	out.lines = append(out.lines, readAttrTree(r.workTree, "")...)
	info := filepath.Join(r.commonDir, "info", "attributes")
	out.lines = append(out.lines, readAttrFile(info, "")...)
	r.attrs = out
	return out
}

// readAttrTree reads every .gitattributes under a directory, deepest last.
func readAttrTree(root, prefix string) []attrLine {
	dir := filepath.Join(root, filepath.FromSlash(prefix))
	out := readAttrFile(filepath.Join(dir, ".gitattributes"), prefix)
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
		out = append(out, readAttrTree(root, child)...)
	}
	return out
}

// readAttrFile parses one attributes file, relative to dir.
func readAttrFile(path, dir string) []attrLine {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []attrLine
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		attrs := map[string]string{}
		for _, token := range fields[1:] {
			switch {
			case strings.HasPrefix(token, "-"):
				attrs[token[1:]] = "unset"
			case strings.HasPrefix(token, "!"):
				attrs[token[1:]] = "unspecified"
			default:
				name, value, hasValue := strings.Cut(token, "=")
				if hasValue {
					attrs[name] = value
				} else {
					attrs[name] = "set"
				}
			}
		}
		out = append(out, attrLine{pattern: fields[0], relative: dir, attrs: attrs})
	}
	return out
}
