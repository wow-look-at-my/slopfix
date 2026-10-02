// referents.go carries the tier that does not read the wording at all: a
// comment naming a symbol that exists nowhere is describing a tree that is
// gone. "see TestDarwinStatfsToLinux for the pin", written beside the change
// that deleted that test, is a tombstone with no tell in its phrasing, and no
// rewording makes it true again.
//
// Precision comes entirely from which identifiers are eligible. A comment about
// low-level work is full of names the repository does not define, and reporting
// those is how a guard earns the reputation that gets it turned off. So a name
// that is all capitals is never a candidate, and neither is a short name.
package tombstones

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wow-look-at-my/go-containers/set"
)

// identifierWords splits text into the runs the shape test judges, by cutting
// on every character an identifier cannot hold.
func identifierWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !(r == '_' ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9'))
	})
}

// minCandidate is the shortest name this rule judges: shorter reads as a word.
const minCandidate = 8

// isCandidate reports whether this repository must contain the name.
func isCandidate(name string) bool {
	return len(name) >= minCandidate &&
		name != strings.ToUpper(name) &&
		identifierShaped(name)
}

// identifierShaped reports whether a whole word is built like a symbol rather
// than prose: it carries an underscore, or a case change inside it.
func identifierShaped(name string) bool {
	if strings.Contains(name, "_") {
		return true
	}
	for i := 1; i < len(name); i++ {
		prev, c := name[i-1], name[i]
		if c >= 'A' && c <= 'Z' && (prev >= 'a' && prev <= 'z' || prev >= '0' && prev <= '9') {
			return true
		}
	}
	// Prose capitalises a sentence head too, so a capital needs a digit.
	return name[0] >= 'A' && name[0] <= 'Z' && strings.ContainsAny(name, "0123456789")
}

// probeTimeout bounds the search, because a guard that hangs is worse.
const probeTimeout = 2 * time.Second

// maxNames bounds what a comment may put to the repository.
const maxNames = 40

// probeBudget is how many files a process probes name by name before it reads the tree once.
const probeBudget = 50

// indexTimeout bounds the read of the tree.
const indexTimeout = 5 * time.Minute

// symbolIndex holds every identifier-shaped word a tree contains.
type symbolIndex struct {
	once  sync.Once
	names set.Set[string]
	ok    bool
}

var (
	indexes sync.Map
	probed  atomic.Int64
)

// indexFor answers the index of root, built on the first call past the budget.
func indexFor(root, rg string) *symbolIndex {
	v, _ := indexes.LoadOrStore(root, &symbolIndex{})
	ix := v.(*symbolIndex)
	if probed.Add(1) <= probeBudget {
		return ix
	}
	ix.once.Do(func() { ix.build(root, rg) })
	return ix
}

// build reads every candidate word in the tree with one ripgrep run.
func (ix *symbolIndex) build(root, rg string) {
	ctx, cancel := context.WithTimeout(context.Background(), indexTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, rg, "--no-messages", "--only-matching", "--no-filename", "--no-line-number", "-e", "[A-Za-z0-9_]{8,}", root)
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return
	}
	names := set.New[string]()
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		if word := scanner.Text(); isCandidate(word) {
			names.Add(word)
		}
	}
	err = cmd.Wait()
	if ctx.Err() != nil || scanner.Err() != nil {
		return
	}
	if err != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() > 1 {
		return
	}
	ix.names, ix.ok = names, true
}

// holds reports a name the index saw.
func (ix *symbolIndex) holds(name string) bool {
	return ix.ok && ix.names.Contains(name)
}

// DeadReferents returns the identifiers the blocks name that appear neither in
// the text nor in the repository. It returns nothing when it cannot answer.
func DeadReferents(path, added string, blocks []Block) []string {
	root := RepoRoot(path)
	if root == "" {
		return nil
	}
	rg, err := exec.LookPath("rg")
	if err != nil {
		return nil
	}

	names := set.New[string]()
	for _, b := range blocks {
		for _, m := range identifierWords(b.Text) {
			if isCandidate(m) {
				names.Add(m)
			}
		}
	}
	if names.Len() == 0 || names.Len() > maxNames {
		return nil
	}

	var ordered []string
	for n := range names.All() {
		if strings.Count(added, n) > 1 {
			continue
		}
		ordered = append(ordered, n)
	}
	if len(ordered) == 0 {
		return nil
	}

	ix := indexFor(root, rg)
	args := []string{"--no-messages", "--fixed-strings", "--files-with-matches", "--max-count", "1"}
	var dead []string
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	for _, name := range ordered {
		if ix.holds(name) {
			continue
		}
		cmd := exec.CommandContext(ctx, rg, append(append([]string{}, args...), "-e", name, root)...)
		out, err := cmd.Output()
		if ctx.Err() != nil {
			return nil
		}
		if err != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() > 1 {
			return nil // ripgrep failed rather than found nothing
		}
		if strings.TrimSpace(string(out)) == "" {
			dead = append(dead, name)
		}
	}
	return dead
}

// RepoRoot walks up from path looking for a working tree. An empty result puts
// path outside every tree.
func RepoRoot(path string) string {
	dir := path
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && (fi.IsDir() || fi.Mode().IsRegular()) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
