// paths.go is the location half of the table. A command rule asks what a call
// RUNS. A path rule asks where it reaches, whichever tool asks.
//
// It is a setting rather than a rule of its own: the tree to refuse is named in
// rules.xml beside everything else this hook decides from.
package autoallow

import (
	"os"
	"path/filepath"
	"strings"
)

// PathRule refuses a call that reaches a tree, whichever tool asks.
type PathRule struct {
	Prefix  string
	Message string
}

// pathFields are the input keys that carry a location. Judging by key rather
// than by value is what keeps a document that merely NAMES the tree writable.
func (t ToolInput) pathFields() []string {
	return []string{t.FilePath, t.NotebookPath, t.Path, t.Pattern}
}

// matchPathRule answers with the message of whichever rule the call reaches.
func matchPathRule(hi HookInput, rules []PathRule) string {
	for _, rule := range rules {
		root := expandHome(rule.Prefix)
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		for _, field := range hi.ToolInput.pathFields() {
			if underRoot(root, hi.Cwd, field) {
				return rule.Message
			}
		}
		// A shell command names its locations as bare words, so the text is
		// scanned whole rather than by key.
		if commandReaches(root, hi.ToolInput.Command) {
			return rule.Message
		}
	}
	return ""
}

// underRoot reports whether a path lands in the tree. A relative path resolves
// against the call's own working directory, which is what the tool would do.
func underRoot(root, cwd, path string) bool {
	if path == "" {
		return false
	}
	p := expandHome(path)
	if !filepath.IsAbs(p) {
		if cwd == "" {
			return false
		}
		p = filepath.Join(cwd, p)
	}
	p = filepath.Clean(p)
	return p == root || strings.HasPrefix(p, root+string(filepath.Separator))
}

// commandReaches reports whether a shell command names the tree. The scan is
// over the raw text: the hook sees the command before the shell has resolved
// anything, so each spelling of the home directory is matched as written.
func commandReaches(root, command string) bool {
	if command == "" {
		return false
	}
	for _, spelling := range homeSpellings(root) {
		if strings.Contains(command, spelling) {
			return true
		}
	}
	return false
}

func homeSpellings(root string) []string {
	home := homeDir()
	if home == "" || !strings.HasPrefix(root, home+string(filepath.Separator)) {
		return []string{root}
	}
	rest := strings.TrimPrefix(root, home)
	return []string{root, "~" + rest, "$HOME" + rest, "${HOME}" + rest}
}

// expandHome resolves the home spellings a tool accepts. An unresolvable home
// answers empty, which allows: a guard that refuses because it could not find a
// home directory is worse than no guard.
func expandHome(path string) string {
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "~") && !strings.HasPrefix(path, "$HOME") && !strings.HasPrefix(path, "${HOME}") {
		return path
	}
	home := homeDir()
	if home == "" {
		return ""
	}
	switch {
	case path == "~" || path == "$HOME" || path == "${HOME}":
		return home
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(home, path[2:])
	case strings.HasPrefix(path, "$HOME/"):
		return filepath.Join(home, path[len("$HOME/"):])
	case strings.HasPrefix(path, "${HOME}/"):
		return filepath.Join(home, path[len("${HOME}/"):])
	}
	return path
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}
