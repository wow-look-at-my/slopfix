package slopfix

import "github.com/wow-look-at-my/slopfix/ste"

// repo/agents-file: a root CLAUDE.md that holds instructions of its own. Every
// agent but Claude Code reads AGENTS.md, so the instructions move there. The
// repair writes AGENTS.md and leaves the import behind in CLAUDE.md.
func init() {
	RegisterRule(RuleSpec{
		ID:       IDAgentsFile,
		Category: RuleRepo,
		Detect:   detectAgentsFile,
		Autofix:  autofixAgentsFile,
		Cases:    []RuleCase{{Name: IDAgentsFile, Files: map[string]string{ClaudeFile: "# Rules\n\nDo the thing.\n"}}},
	})
}

// detectAgentsFile answers every root CLAUDE.md that carries more than the import.
func detectAgentsFile(c RuleCase) []ste.Finding { return treeFindings(c, IDAgentsFile) }

// autofixAgentsFile moves the instructions into AGENTS.md.
func autofixAgentsFile(c RuleCase) RuleCase { return treeAutofix(c, IDAgentsFile) }
