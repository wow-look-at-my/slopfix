package gitread

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// config maps a dotted key to its last value. The section and the name are
// lower case. A subsection keeps its case.
type config map[string]string

// maxIncludeDepth stops an include cycle.
const maxIncludeDepth = 8

// Config answers a configuration value, or "" when it is unset. It reads the
// system, global, repository and work tree files, and GIT_CONFIG_COUNT, as git does.
func (r *Repo) Config(key string) string {
	r.cfgOnce.Do(func() { r.cfg = r.loadConfig() })
	return r.cfg[strings.ToLower(key)]
}

// ConfigBool answers a boolean value, or def when it is unset.
func (r *Repo) ConfigBool(key string, def bool) bool {
	switch strings.ToLower(r.Config(key)) {
	case "":
		return def
	case "true", "yes", "on", "1":
		return true
	case "false", "no", "off", "0":
		return false
	}
	return def
}

// OriginURL answers the URL the origin remote clones from, or "".
func (r *Repo) OriginURL() string { return r.Config("remote.origin.url") }

// PromisorRemote answers the remote a partial clone fetches a missing object
// from, or "" for a complete repository.
func (r *Repo) PromisorRemote() string {
	if name := r.Config("extensions.partialclone"); name != "" {
		return name
	}
	var names []string
	for key, value := range r.cfg {
		rest, ok := strings.CutPrefix(key, "remote.")
		if name, promisor := strings.CutSuffix(rest, ".promisor"); ok && promisor && truthy(value) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func (r *Repo) loadConfig() config {
	out := config{}
	merge := func(c config) {
		for k, v := range c {
			out[k] = v
		}
	}
	if !truthy(os.Getenv("GIT_CONFIG_NOSYSTEM")) {
		path := os.Getenv("GIT_CONFIG_SYSTEM")
		if path == "" {
			path = "/etc/gitconfig"
		}
		merge(readConfigFile(path, 0))
	}
	for _, path := range globalConfigFiles() {
		merge(readConfigFile(path, 0))
	}
	local := readConfigFile(filepath.Join(r.commonDir, "config"), 0)
	merge(local)
	if local["extensions.worktreeconfig"] == "true" {
		merge(readConfigFile(filepath.Join(r.gitDir, "config.worktree"), 0))
	}
	if n, err := strconv.Atoi(os.Getenv("GIT_CONFIG_COUNT")); err == nil {
		for i := 0; i < n; i++ {
			key := os.Getenv("GIT_CONFIG_KEY_" + strconv.Itoa(i))
			if key != "" {
				out[normalKey(key)] = os.Getenv("GIT_CONFIG_VALUE_" + strconv.Itoa(i))
			}
		}
	}
	return out
}

func globalConfigFiles() []string {
	if path, ok := os.LookupEnv("GIT_CONFIG_GLOBAL"); ok {
		return []string{path}
	}
	var out []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		out = append(out, filepath.Join(xdg, "git", "config"))
	} else if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, ".config", "git", "config"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, ".gitconfig"))
	}
	return out
}

func truthy(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// normalKey lowers the section and the name of a dotted key, not the subsection.
func normalKey(key string) string {
	first := strings.IndexByte(key, '.')
	last := strings.LastIndexByte(key, '.')
	if first < 0 {
		return strings.ToLower(key)
	}
	return strings.ToLower(key[:first]) + key[first:last] + strings.ToLower(key[last:])
}

// ConfigFile parses one file in git config syntax, such as .gitmodules.
func ConfigFile(path string) map[string]string { return readConfigFile(path, 0) }

// readConfigFile parses one git config file and the files it includes.
func readConfigFile(path string, depth int) config {
	out := config{}
	data, err := os.ReadFile(path)
	if err != nil || depth > maxIncludeDepth {
		return out
	}
	section := ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				continue
			}
			section = sectionKey(line[1:end])
			line = strings.TrimSpace(line[end+1:])
			if line == "" {
				continue
			}
		}
		name, value, hasValue := strings.Cut(line, "=")
		name = strings.ToLower(strings.TrimSpace(name))
		if !hasValue {
			value = "true"
		} else {
			value = configValue(value)
		}
		key := name
		if section != "" {
			key = section + "." + name
		}
		if key == "include.path" {
			for k, v := range readConfigFile(includePath(path, value), depth+1) {
				out[k] = v
			}
			continue
		}
		out[key] = value
	}
	return out
}

// includePath resolves an include relative to the file that names it.
func includePath(from, path string) string {
	path = expandHome(path)
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(from), path)
}

// sectionKey turns a section header into its dotted key.
func sectionKey(header string) string {
	header = strings.TrimSpace(header)
	if name, sub, ok := strings.Cut(header, " "); ok {
		sub = strings.TrimSpace(sub)
		sub = strings.TrimSuffix(strings.TrimPrefix(sub, `"`), `"`)
		sub = strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(sub)
		return strings.ToLower(strings.TrimSpace(name)) + "." + sub
	}
	// The old spelling [section.sub] lowers the subsection too.
	return strings.ToLower(header)
}

// configValue unquotes a value and drops a trailing comment.
func configValue(raw string) string {
	var b strings.Builder
	quoted := false
	raw = strings.TrimSpace(raw)
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == '"':
			quoted = !quoted
		case c == '\\' && i+1 < len(raw):
			i++
			switch raw[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(raw[i])
			}
		case (c == '#' || c == ';') && !quoted:
			return strings.TrimSpace(b.String())
		default:
			b.WriteByte(c)
		}
	}
	return strings.TrimSpace(b.String())
}

// expandHome expands a leading ~/ the way git expands a path value.
func expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return path
}
