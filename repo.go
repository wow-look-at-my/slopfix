package slopfix

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/ste"
)

// The repository rules. They judge the tree rather than a file, so only a walk
// from the repository root reaches them.
const (
	// IDAgentsFile is a root CLAUDE.md that holds more than the AGENTS.md import.
	IDAgentsFile = "repo/agents-file"
	// IDBudget is a root file, a CLAUDE.md or an imported snippet past CharBudget.
	IDBudget = "repo/budget"
)

// RuleRepo names the repository rules as a category.
const RuleRepo Rule = "repo"

// RepoIDs names every repository rule.
var RepoIDs = set.Of(IDAgentsFile, IDBudget)

// isRepoRoot reports whether dir is the top of a repository.
func isRepoRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// repoRun reports what the repository rules find under root. When writing, it
// also applies the repairs the caller keeps, and names each file it changed.
func repoRun(root string, keeps func(string) bool, writing bool) (findings []TreeFinding, changed []string, err error) {
	plan, err := SurveyRoot(root, true)
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
	if !keeps(IDBudget) {
		return findings, changed, nil
	}
	// The move above can grow AGENTS.md, so the sizes are measured again.
	if plan, err = SurveyRoot(root, true); err != nil {
		return nil, nil, err
	}
	over := make([]string, 0, len(plan.OverBudget))
	for rel := range plan.OverBudget {
		over = append(over, rel)
	}
	sort.Strings(over)
	for _, rel := range over {
		size := plan.OverBudget[rel]
		if writing {
			// The sections move into a docs/ beside the file, so its links stay relative.
			dir := filepath.Join(root, filepath.Dir(rel))
			written, err := Split(dir, filepath.Base(rel), false)
			if err != nil {
				return nil, nil, err
			}
			if len(written) > 0 {
				changed = append(changed, filepath.Join(root, rel))
				for _, name := range written {
					changed = append(changed, filepath.Join(dir, filepath.FromSlash(name)))
				}
			}
			if size, err = charCount(filepath.Join(root, rel)); err != nil {
				return nil, nil, err
			}
			if size <= CharBudget {
				continue
			}
		}
		findings = append(findings, repoFinding(rel, IDBudget,
			fmt.Sprintf("%d characters, over the %d budget every request pays for", size, CharBudget),
			"`slopfix fix` moves its largest `##` sections into docs/. A file with no such section needs a cut by hand."))
	}
	return findings, changed, nil
}

func repoFinding(path, id, rule, fix string) TreeFinding {
	return TreeFinding{Path: path, Finding: ste.Finding{Line: 1, ID: id, Rule: rule, Fix: fix}}
}
