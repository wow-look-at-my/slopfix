package gitread

import (
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Attribute values, as git check-attr prints them.
const (
	AttrSet         = "set"
	AttrUnset       = "unset"
	AttrUnspecified = "unspecified"
)

// attrLine is one gitattributes line: a pattern and what it assigns.
type attrLine struct {
	pattern
	attrs []attrValue
}

type attrValue struct {
	name  string
	value string
}

// builtinMacros are the macros git defines itself.
var builtinMacros = map[string][]attrValue{
	"binary": {{"binary", AttrSet}, {"diff", AttrUnset}, {"merge", AttrUnset}, {"text", AttrUnset}},
}

// Attributes answers attribute values, as git check-attr does. Its rules come
// from each .gitattributes on the way down, info/attributes, and
// core.attributesFile. It reads each directory's rules once.
type Attributes struct {
	repo   *Repo
	fold   bool
	global []attrLine
	info   []attrLine
	macros map[string][]attrValue

	mu   sync.Mutex
	dirs map[string][]attrLine
}

// Attributes answers a matcher that reads the rule files as they stand now.
func (r *Repo) Attributes() *Attributes {
	a := &Attributes{repo: r, fold: r.ConfigBool("core.ignorecase", false), dirs: map[string][]attrLine{}}
	if global := r.globalFile("core.attributesfile", "attributes"); global != "" {
		a.global = load(&r.rules.attrs, global, parseAttrs(""))
	}
	a.info = load(&r.rules.attrs, filepath.Join(r.commonDir, "info", "attributes"), parseAttrs(""))
	a.macros = map[string][]attrValue{}
	for name, values := range builtinMacros {
		a.macros[name] = values
	}
	// A macro may be defined only where every path sees it: the root file,
	// info/attributes, and the global file.
	for _, src := range [][]attrLine{a.global, a.rulesIn(""), a.info} {
		for _, line := range src {
			if name, ok := strings.CutPrefix(line.glob, "[attr]"); ok {
				a.macros[name] = line.attrs
			}
		}
	}
	return a
}

// Attr answers one attribute's value for a path relative to the work tree.
func (r *Repo) Attr(rel, name string) string { return r.Attributes().Value(rel, name) }

// AttributeSet reports a value that turns an attribute on.
func AttributeSet(value string) bool {
	switch value {
	case AttrUnspecified, AttrUnset, "false":
		return false
	}
	return true
}

func parseAttrs(base string) func([]byte) []attrLine {
	return func(data []byte) []attrLine {
		var out []attrLine
		for _, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
			if line == "" || line[0] == '#' {
				continue
			}
			glob, rest := cutPattern(line)
			if glob == "" || glob[0] == '!' {
				// Git refuses a negative pattern in gitattributes.
				continue
			}
			var values []attrValue
			for _, token := range strings.Fields(rest) {
				switch {
				case token[0] == '-':
					values = append(values, attrValue{token[1:], AttrUnset})
				case token[0] == '!':
					values = append(values, attrValue{token[1:], AttrUnspecified})
				default:
					name, value, ok := strings.Cut(token, "=")
					if !ok {
						value = AttrSet
					}
					values = append(values, attrValue{name, value})
				}
			}
			if strings.HasPrefix(glob, "[attr]") {
				out = append(out, attrLine{pattern: pattern{glob: glob}, attrs: values})
				continue
			}
			dirOnly := strings.HasSuffix(glob, "/")
			p := newPattern(strings.TrimSuffix(glob, "/"), base)
			// A pattern for a directory never matches a file in gitattributes.
			p.dirOnly = dirOnly
			out = append(out, attrLine{pattern: p, attrs: values})
		}
		return out
	}
}

// cutPattern splits a line into its pattern, which may be C-quoted, and the rest.
func cutPattern(line string) (string, string) {
	if line[0] == '"' {
		for i := 1; i < len(line); i++ {
			if line[i] == '\\' {
				i++
				continue
			}
			if line[i] == '"' {
				if glob, err := strconv.Unquote(line[:i+1]); err == nil {
					return glob, line[i+1:]
				}
				break
			}
		}
	}
	end := strings.IndexAny(line, " \t")
	if end < 0 {
		return line, ""
	}
	return line[:end], line[end:]
}

// rulesIn answers the lines of dir's .gitattributes.
func (a *Attributes) rulesIn(dir string) []attrLine {
	a.mu.Lock()
	rules, ok := a.dirs[dir]
	a.mu.Unlock()
	if ok {
		return rules
	}
	rules = load(&a.repo.rules.attrs, filepath.Join(a.repo.workTree, filepath.FromSlash(dir), ".gitattributes"), parseAttrs(dir))
	a.mu.Lock()
	a.dirs[dir] = rules
	a.mu.Unlock()
	return rules
}

// Value answers one attribute's value: "set", "unset", "unspecified", or the
// text after "=". info/attributes wins, then the nearest .gitattributes, then
// the global file. In one file the last matching line wins.
func (a *Attributes) Value(rel, name string) string {
	if a.repo.workTree == "" || rel == "" {
		return AttrUnspecified
	}
	base := baseOf(rel)
	if v, ok := a.decide(a.info, rel, base, name); ok {
		return v
	}
	for dir := dirOf(rel); ; dir = dirOf(dir) {
		if v, ok := a.decide(a.rulesIn(dir), rel, base, name); ok {
			return v
		}
		if dir == "" {
			break
		}
	}
	if v, ok := a.decide(a.global, rel, base, name); ok {
		return v
	}
	return AttrUnspecified
}

func (a *Attributes) decide(lines []attrLine, rel, base, name string) (string, bool) {
	for i := len(lines) - 1; i >= 0; i-- {
		line := &lines[i]
		if strings.HasPrefix(line.glob, "[attr]") || !line.matches(rel, base, false, a.fold) {
			continue
		}
		if v, ok := a.assigned(line.attrs, name, 0); ok {
			return v, true
		}
	}
	return "", false
}

// maxMacroDepth stops a macro cycle.
const maxMacroDepth = 8

// assigned answers the value a line gives name, through macros. A later
// token on the line wins.
func (a *Attributes) assigned(values []attrValue, name string, depth int) (string, bool) {
	for i := len(values) - 1; i >= 0; i-- {
		v := values[i]
		if v.name == name {
			return v.value, true
		}
		if expansion, ok := a.macros[v.name]; ok && v.value == AttrSet && depth < maxMacroDepth {
			if got, ok := a.assigned(expansion, name, depth+1); ok {
				return got, true
			}
		}
	}
	return "", false
}
