package slopfix

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
)

// The repository rules. They judge the tree rather than a file, so only a walk
// from the repository root reaches them.
const (
	// IDStrayMarkdown is a markdown file other than the kept root files.
	IDStrayMarkdown = "repo/stray-markdown"
	// IDAgentsFile is a root CLAUDE.md that holds more than the AGENTS.md import.
	IDAgentsFile = "repo/agents-file"
	// IDBudget is a kept file past CharBudget.
	IDBudget = "repo/budget"
)

// RuleRepo names the repository rules as a category.
const RuleRepo Rule = "repo"

// RepoIDs names every repository rule.
var RepoIDs = set.Of(IDStrayMarkdown, IDAgentsFile, IDBudget)

// isRepoRoot reports whether dir is the top of a repository.
func isRepoRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// repoRun reports what the repository rules find under root. When writing, it
// also applies the repairs the caller keeps, and names each file it changed.
func repoRun(root string, keeps func(string) bool, writing bool) (findings []TreeFinding, changed []string, err error) {
	plan, err := Purge(root, true)
	if err != nil {
		return nil, nil, err
	}
	if plan.Agents.Changed() && keeps(IDAgentsFile) {
		if !writing {
			findings = append(findings, repoFinding(ClaudeFile, IDAgentsFile,
				"CLAUDE.md holds instructions, and every agent but Claude Code reads AGENTS.md",
				"Move them into AGENTS.md. `slopfix fix` does this."))
		} else if _, err := MigrateAgents(root, false); err != nil {
			return nil, nil, err
		} else {
			changed = append(changed, filepath.Join(root, ClaudeFile))
		}
	}
	for _, rel := range plan.Deleted {
		if !keeps(IDStrayMarkdown) {
			break
		}
		if !writing {
			findings = append(findings, repoFinding(rel, IDStrayMarkdown,
				"a repository keeps no markdown but README.md, AGENTS.md and CLAUDE.md at its root",
				"Move what it says into AGENTS.md or README.md. `slopfix fix` deletes it."))
			continue
		}
		if err := os.Remove(filepath.Join(root, rel)); err != nil {
			return nil, nil, err
		}
		changed = append(changed, filepath.Join(root, rel))
	}
	for rel, size := range plan.OverBudget {
		if keeps(IDBudget) {
			findings = append(findings, repoFinding(rel, IDBudget,
				fmt.Sprintf("%d characters, over the %d budget every request pays for", size, CharBudget),
				"Cut it down."))
		}
	}
	return findings, changed, nil
}

func repoFinding(path, id, rule, fix string) TreeFinding {
	return TreeFinding{Path: path, Finding: ste.Finding{Line: 1, ID: id, Rule: rule, Fix: fix}}
}
