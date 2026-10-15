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
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

// foreignInGo reports an underscored name in Go, which is a C name or a file.
// A test function keeps its underscore and stays a candidate.
func foreignInGo(path, name string) bool {
	if !strings.HasSuffix(path, ".go") || !strings.Contains(name, "_") {
		return false
	}
	for _, prefix := range []string{"Test", "Benchmark", "Example", "Fuzz"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
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
		for _, m := range identifierWords(withoutURLs(b.Prose)) {
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

	ix := indexFor(root)
	if ix.missingDependency(root) {
		return nil
	}
	var ordered []string
	for n := range names.All() {
		if strings.Count(added, n) > 1 || foreignInGo(path, n) || ix.namesAFile(root, n) {
			continue
		}
		ordered = append(ordered, n)
	}
	if len(ordered) == 0 {
		return nil
	}

	self, _ := os.ReadFile(path)
	if ix.portsForeignSource(root, path, blocks, self) {
		return nil
	}
	selfPath, _ := filepath.Abs(path)
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

// foreignCitation matches a C family source cited at a line, as in Script.h:222.
var foreignCitation = regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_.-]*\.(?:c|cc|cpp|cxx|h|hh|hpp|hxx|inl|m|mm):[0-9]+\b`)

// portsForeignSource reports a file whose comments cite a line of a source
// file that the repository does not hold. Its test counterpart counts too.
// Such a file ports another codebase, so its comments name that codebase's
// API, and the repository has no definition to find.
func (ix *symbolIndex) portsForeignSource(root, path string, blocks []Block, self []byte) bool {
	ix.listPaths(root)
	if !ix.pathsOK {
		return false
	}
	if ix.citesForeign(blocks) || ix.citesForeign(AddedBlocks(path, string(self))) {
		return true
	}
	other := testCounterpart(path)
	if other == "" {
		return false
	}
	data, err := os.ReadFile(other)
	return err == nil && ix.citesForeign(AddedBlocks(other, string(data)))
}

// citesForeign reports a block that cites a line of a file no listed path names.
func (ix *symbolIndex) citesForeign(blocks []Block) bool {
	for _, b := range blocks {
		for _, cite := range foreignCitation.FindAllString(b.Prose, -1) {
			file, _, _ := strings.Cut(cite, ":")
			if !ix.bases.Contains(file) {
				return true
			}
		}
	}
	return false
}

// testCounterpart answers the source file a test file tests, or the test file
// of a source file. X_test.go and x.go, x.test.ts and x.ts.
func testCounterpart(path string) string {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for _, mark := range []string{"_test", ".test", ".spec"} {
		if base, ok := strings.CutSuffix(stem, mark); ok {
			return base + ext
		}
	}
	if ext == ".go" {
		return stem + "_test.go"
	}
	return stem + ".test" + ext
}
