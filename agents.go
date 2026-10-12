package slopfix

import (
	"os"
	"path/filepath"
	"strings"
)

// AgentsFile holds a repository's agent instructions. Every agent reads it by name.
const AgentsFile = "AGENTS.md"

// ClaudeFile is the name Claude Code reads. It stays as an import of AgentsFile.
const ClaudeFile = "CLAUDE.md"

// ClaudeStub is the whole of CLAUDE.md after a migration.
const ClaudeStub = "@" + AgentsFile + "\n"

// Migration is what MigrateAgents did to the root pair, or what it would do.
type Migration struct {
	// Renamed is true when CLAUDE.md became AGENTS.md.
	Renamed bool
	// Merged is true when CLAUDE.md was appended to an existing AGENTS.md.
	Merged bool
}

// Changed reports whether the migration touched a file.
func (m Migration) Changed() bool { return m.Renamed || m.Merged }

// MigrateAgents moves a root CLAUDE.md into AGENTS.md and leaves CLAUDE.md as
// the import line. A CLAUDE.md that is a symlink, or already the import, stays.
func MigrateAgents(root string, dryRun bool) (Migration, error) {
	claudePath := filepath.Join(root, ClaudeFile)
	agentsPath := filepath.Join(root, AgentsFile)

	info, err := os.Lstat(claudePath)
	if os.IsNotExist(err) {
		return Migration{}, nil
	}
	if err != nil {
		return Migration{}, err
	}
	if !info.Mode().IsRegular() {
		return Migration{}, nil
	}
	raw, err := os.ReadFile(claudePath)
	if err != nil {
		return Migration{}, err
	}
	body := strings.TrimSpace(withoutImport(string(raw)))
	if body == "" {
		return Migration{}, nil
	}

	agents, err := os.ReadFile(agentsPath)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return Migration{}, err
	}

	var merged string
	var result Migration
	switch {
	case !exists:
		merged, result = body+"\n", Migration{Renamed: true}
	case strings.Contains(string(agents), body):
		merged, result = string(agents), Migration{Merged: true}
	default:
		merged, result = strings.TrimRight(string(agents), "\n")+"\n\n"+body+"\n", Migration{Merged: true}
	}
	if dryRun {
		return result, nil
	}
	if merged != string(agents) || !exists {
		if err := os.WriteFile(agentsPath, []byte(merged), info.Mode().Perm()); err != nil {
			return Migration{}, err
		}
	}
	if err := os.WriteFile(claudePath, []byte(ClaudeStub), info.Mode().Perm()); err != nil {
		return Migration{}, err
	}
	return result, nil
}

// withoutImport drops every line that only imports AGENTS.md.
func withoutImport(text string) string {
	lines := strings.Split(text, "\n")
	out := lines[:0]
	for _, line := range lines {
		if strings.TrimSpace(line) == "@"+AgentsFile || strings.TrimSpace(line) == "@./"+AgentsFile {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
