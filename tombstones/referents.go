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
	"bytes"
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

// indexFileCap is the largest tracked file the index reads. A larger one is data.
const indexFileCap = 4 << 20

// symbolIndex holds every identifier-shaped word the tracked files contain.
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
func indexFor(root string) *symbolIndex {
	v, _ := indexes.LoadOrStore(root, &symbolIndex{})
	ix := v.(*symbolIndex)
	if probed.Add(1) <= probeBudget {
		return ix
	}
	ix.once.Do(func() { ix.build(root) })
	return ix
}

// build reads every candidate word in the files git tracks under root. The
// tracked set leaves out a nested checkout and built output, as the walk does.
func (ix *symbolIndex) build(root string) {
	ctx, cancel := context.WithTimeout(context.Background(), indexTimeout)
	defer cancel()
	listed, err := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return
	}
	names := set.New[string]()
	for _, rel := range strings.Split(string(listed), "\x00") {
		if rel == "" {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil || len(data) > indexFileCap || bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
			continue
		}
		addWords(names, data)
	}
	ix.names, ix.ok = names, true
}

// addWords adds every candidate identifier in data to names.
func addWords(names set.Set[string], data []byte) {
	start := -1
	for i, c := range data {
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 && i-start >= minCandidate {
			if word := string(data[start:i]); isCandidate(word) {
				names.Add(word)
			}
		}
		start = -1
	}
	if start >= 0 && len(data)-start >= minCandidate {
		if word := string(data[start:]); isCandidate(word) {
			names.Add(word)
		}
	}
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

	ix := indexFor(root)
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
