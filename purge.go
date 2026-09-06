package slopfmt

import (
	"fmt"
	"github.com/wow-look-at-my/go-containers/set"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// A repository keeps README.md for a person arriving at the repo, and CLAUDE.md
// for an agent working in it. Both live at the root.
//
// Every other .md is deleted on sight. The prose in them is written far more
// often than it is read, it drifts away from the code within days, and it costs
// its reader more than it returns. A spec repository is the exception, because
// there the prose IS the product.
var kept = set.Of[string]("README.md", "CLAUDE.md")

// CharBudget caps each kept file. Past it, nobody skims the file, and every
// request pays for the whole thing.
const CharBudget = 40_000

// skipDirs are never walked: their contents belong to somebody else.
var skipDirs = set.Of[string](".git", "node_modules", "vendor", "dist")

// PurgeResult is what a purge did, or what it would do.
type PurgeResult struct {
	// Deleted are the files removed, relative to the root, in walk order.
	Deleted []string
	// OverBudget names each kept file past CharBudget, with its size.
	OverBudget map[string]int
}

// Purge deletes every markdown file under root except the kept files at its own
// top level. A dry run reports the same list and removes nothing.
//
// A spec repository opts out with a .slopfmt-spec marker at its root, which is
// the only exemption. Deciding by marker rather than by a list in this repo
// keeps the answer with the repository it describes.
func Purge(root string, dryRun bool) (*PurgeResult, error) {
	result := &PurgeResult{OverBudget: map[string]int{}}
	if spec, err := isSpecRepo(root); err != nil || spec {
		return result, err
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if entry.IsDir() {
			if rel != "." && skipDirs.Contains(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(entry.Name())) != ".md" {
			return nil
		}
		if kept.Contains(entry.Name()) && !strings.Contains(rel, string(filepath.Separator)) {
			return budgetOf(path, rel, result)
		}
		result.Deleted = append(result.Deleted, rel)
		if dryRun {
			return nil
		}
		return os.Remove(path)
	})
	return result, err
}

// budgetOf records a kept file that is past the budget. The count is characters,
// not bytes: a check that counts bytes reports a file with an em dash as longer
// than it reads.
func budgetOf(path, rel string, result *PurgeResult) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if size := len([]rune(string(content))); size > CharBudget {
		result.OverBudget[rel] = size
	}
	return nil
}

// isSpecRepo reports whether the root opts out of the purge.
func isSpecRepo(root string) (bool, error) {
	_, err := os.Stat(filepath.Join(root, ".slopfmt-spec"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// BudgetError renders the over-budget files as a single message.
func BudgetError(over map[string]int) string {
	var b strings.Builder
	for name, size := range over {
		fmt.Fprintf(&b, "%s: %d characters, over the %d budget. Cut it down.\n", name, size, CharBudget)
	}
	return b.String()
}
