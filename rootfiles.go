package slopfix

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The root files every request loads. CharBudget caps each.
var rootFiles = []string{"README.md", AgentsFile, ClaudeFile}

// CharBudget caps each root file, because every request pays for the whole file.
const CharBudget = 40_000

// RootResult is what the repository rules found, or did, at the root.
type RootResult struct {
	// OverBudget names each root file past CharBudget, with its size.
	OverBudget map[string]int
	// Agents is what the move of CLAUDE.md into AGENTS.md did.
	Agents Migration
}

// SurveyRoot moves CLAUDE.md into AGENTS.md, then measures each root file
// against CharBudget. A dry run reports the same work and changes nothing.
func SurveyRoot(root string, dryRun bool) (*RootResult, error) {
	result := &RootResult{OverBudget: map[string]int{}}
	agents, err := MigrateAgents(root, dryRun)
	if err != nil {
		return result, err
	}
	result.Agents = agents
	for _, name := range rootFiles {
		if err := budgetOf(filepath.Join(root, name), name, result); err != nil {
			return result, err
		}
	}
	return result, nil
}

// budgetOf records a root file that is past the budget. The count is characters,
// not bytes: a check that counts bytes reports a file with an em dash as longer
// than it reads. A root file that does not exist has no size to judge.
func budgetOf(path, rel string, result *RootResult) error {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if size := len([]rune(string(content))); size > CharBudget {
		result.OverBudget[rel] = size
	}
	return nil
}

// BudgetError renders the over-budget files as a single message.
func BudgetError(over map[string]int) string {
	var b strings.Builder
	for name, size := range over {
		fmt.Fprintf(&b, "%s: %d characters, over the %d budget. Cut it down.\n", name, size, CharBudget)
	}
	return b.String()
}
