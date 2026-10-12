// referents.go carries the tier that does not read the wording at all: a
// comment naming a symbol that exists nowhere is describing a tree. That tree
// is gone. "see TestDarwinStatfsToLinux for the pin", written beside the
// change that deleted that test, is a tombstone with no tell in its phrasing,
// and no rewording makes it true again.
//
// Precision comes entirely from which identifiers are eligible. Consider a
// comment about low-level work. That comment is full of names the repository
// does not define. Reporting those is how a guard earns the reputation that
// gets it turned off. So a name that is all capitals is never a candidate, and
// neither is a short name.
package tombstones

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitmod"
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
// than prose: it carries an underscore, or a case change inside it. A word
// that starts with a digit is a number literal, never a symbol.
func identifierShaped(name string) bool {
	if name[0] >= '0' && name[0] <= '9' {
		return false
	}
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

// documentName answers the part of a bare document word that can name a
// symbol, or "".
func documentName(word string) string {
	word = strings.Trim(word, "_")
	if !strings.Contains(word, "_") {
		return ""
	}
	return word
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

// symbolIndex holds every identifier-shaped word the tracked files contain, with the count of files that hold it.
type symbolIndex struct {
	once  sync.Once
	files map[string]int
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

// PrimeIndex builds the index of the working tree that holds path now. A tree
// walk calls it first, because it probes far past the budget.
func PrimeIndex(path string) {
	root := RepoRoot(path)
	if root == "" {
		return
	}
	v, _ := indexes.LoadOrStore(root, &symbolIndex{})
	ix := v.(*symbolIndex)
	ix.once.Do(func() { ix.build(root) })
	probed.Add(probeBudget)
}

// build reads every candidate word in the files git lists under root: tracked,
// and untracked unless git ignores them. That is what ripgrep reads, without a
// submodule's checkout, which the walk skips too.
func (ix *symbolIndex) build(root string) {
	ctx, cancel := context.WithTimeout(context.Background(), indexTimeout)
	defer cancel()
	listed, err := gitmod.CommandContext(ctx, root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return
	}
	files := map[string]int{}
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
		words := set.New[string]()
		addWords(words, data)
		for word := range words.All() {
			files[word]++
		}
	}
	ix.files, ix.ok = files, true
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
	return ix.ok && ix.files[name] > 0
}

// holdsBeyond reports a name the index saw in a file other than self, where
// self is the copy on disk of the file being judged.
func (ix *symbolIndex) holdsBeyond(name string, self []byte) bool {
	n := ix.files[name]
	if bytes.Contains(self, []byte(name)) {
		n--
	}
	return ix.ok && n > 0
}

// DeadReferents returns the identifiers the blocks name that appear neither in
// the text nor in the repository. It returns nothing when it cannot answer.
//
// The file being judged does not count as the repository. Its copy on disk
// holds the comment itself, and the text says once where else the name sits.
func DeadReferents(path, added string, blocks []Block) []string {
	root := RepoRoot(path)
	if root == "" {
		return nil
	}
	doc := IsDocument(path)
	names := set.New[string]()
	for _, b := range blocks {
		// The names come from the comment alone. Code on a shared line names what it uses, and the repository answers for that.
		for _, m := range identifierWords(b.Prose) {
			if doc {
				m = documentName(m)
			}
			if m != "" && isCandidate(m) {
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

	self, _ := os.ReadFile(path)
	selfPath, _ := filepath.Abs(path)
	ix := indexFor(root)
	rg := ""
	if !ix.ok {
		found, err := exec.LookPath("rg")
		if err != nil {
			// With no ripgrep to probe, the index reads the tree once and answers every name.
			ix.once.Do(func() { ix.build(root) })
		}
		rg = found
	}
	if !ix.ok && rg == "" {
		return nil
	}
	// --word-regexp matches whole identifiers, the way the index counts them.
	args := []string{"--no-messages", "--fixed-strings", "--word-regexp", "--files-with-matches", "--max-count", "1"}
	var dead []string
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	for _, name := range ordered {
		// A built index read every file a probe reads, so a name it never saw is dead.
		if ix.ok {
			if !ix.holdsBeyond(name, self) {
				dead = append(dead, name)
			}
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
		if !slices.ContainsFunc(strings.Split(string(out), "\n"), func(file string) bool {
			abs, _ := filepath.Abs(file)
			return file != "" && abs != selfPath
		}) {
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

// DeadReferentHits answers a hit for each name the comments of text carry
// that no other file of the repository holds.
func DeadReferentHits(path, text string) []Hit {
	blocks := AddedBlocks(path, text)
	var out []Hit
	for _, name := range DeadReferents(path, text, blocks) {
		out = append(out, HitForName(blocks, name))
	}
	return out
}
