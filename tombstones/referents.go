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
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitmod"
)

// ownNames answers the identifier words of prose that can name a symbol of
// this repository. A word inside a URL is part of an address.
func ownNames(prose string) []string {
	prose = urlPattern.ReplaceAllStringFunc(prose, func(u string) string { return strings.Repeat(" ", len(u)) })
	var out []string
	for _, loc := range identifierPattern.FindAllStringIndex(prose, -1) {
		before, word := prose[:loc[0]], prose[loc[0]:loc[1]]
		switch {
		// A qualifier puts the name in another namespace.
		case strings.HasSuffix(before, ".") || strings.HasSuffix(before, "#") || strings.HasSuffix(before, ":"):
		// A possessive gives the name to its owner.
		case strings.HasSuffix(before, "'s ") || strings.HasSuffix(before, "’s "):
		case isPlaceholder(word):
		// A trailing star or underscore makes the name a pattern over many names.
		case strings.HasSuffix(word, "_") || strings.HasPrefix(prose[loc[1]:], "*"):
		default:
			out = append(out, word)
		}
	}
	return out
}

var (
	urlPattern        = regexp.MustCompile(`\S+://\S+`)
	identifierPattern = regexp.MustCompile(`[A-Za-z0-9_]+`)
)

// isPlaceholder reports a name built from a stand-in word, which names no symbol.
func isPlaceholder(word string) bool {
	if strings.Contains(word, "Xxx") {
		return true
	}
	for _, part := range strings.Split(strings.ToLower(word), "_") {
		if part == "foo" || part == "bar" || part == "baz" {
			return true
		}
	}
	return false
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
// untracked unless git ignores them, and tracked in a checked-out submodule.
// That is what ripgrep reads. A name a submodule defines is defined here.
func (ix *symbolIndex) build(root string) {
	ctx, cancel := context.WithTimeout(context.Background(), indexTimeout)
	defer cancel()
	listed, err := listedFiles(ctx, root)
	if err != nil {
		return
	}
	files := map[string]int{}
	for _, rel := range listed {
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
		for _, m := range ownNames(b.Prose) {
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
	stems := fileStems(root)
	// alive answers whether the repository holds name, and false for ok when it cannot answer.
	alive := func(name string) (held, ok bool) {
		// The stem of a file that is here names that file.
		if stems.Contains(name) {
			return true, true
		}
		// A built index read every file a probe reads, so a name it never saw is dead.
		if ix.ok {
			return ix.holdsBeyond(name, self), true
		}
		cmd := exec.CommandContext(ctx, rg, append(append([]string{}, args...), "-e", name, root)...)
		out, err := cmd.Output()
		if ctx.Err() != nil {
			return false, false
		}
		if err != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() > 1 {
			return false, false // ripgrep failed rather than found nothing
		}
		return slices.ContainsFunc(strings.Split(string(out), "\n"), func(file string) bool {
			abs, _ := filepath.Abs(file)
			return file != "" && abs != selfPath
		}), true
	}
	for _, name := range ordered {
		held := false
		for _, form := range nameForms(name) {
			h, ok := alive(form)
			if !ok {
				return nil
			}
			if h {
				held = true
				break
			}
		}
		if !held {
			dead = append(dead, name)
		}
	}
	return dead
}

// nameForms answers name. The spellings prose gives a name it uses: a
// plural with an added s, and a capital at the start of a sentence.
func nameForms(name string) []string {
	forms := []string{name}
	if trimmed, ok := strings.CutSuffix(name, "s"); ok && isCandidate(trimmed) {
		forms = append(forms, trimmed)
	}
	if first := name[0]; first >= 'A' && first <= 'Z' {
		forms = append(forms, string(first+'a'-'A')+name[1:])
	}
	return forms
}

var stemsByRoot sync.Map

// fileStems answers the base name before the first dot of every file git
// lists under root, read once per root.
func fileStems(root string) set.Set[string] {
	if v, ok := stemsByRoot.Load(root); ok {
		return v.(set.Set[string])
	}
	stems := set.New[string]()
	ctx, cancel := context.WithTimeout(context.Background(), indexTimeout)
	defer cancel()
	listed, _ := listedFiles(ctx, root)
	for _, rel := range listed {
		if stem, _, _ := strings.Cut(filepath.Base(rel), "."); stem != "" {
			stems.Add(stem)
		}
	}
	v, _ := stemsByRoot.LoadOrStore(root, stems)
	return v.(set.Set[string])
}

// listedFiles answers the files git lists under root: tracked, untracked
// unless ignored, and tracked in each checked-out submodule.
func listedFiles(ctx context.Context, root string) ([]string, error) {
	top, err := gitmod.CommandContext(ctx, root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, err
	}
	seen := set.New[string]()
	var out []string
	add := func(listed []byte) {
		for _, rel := range strings.Split(string(listed), "\x00") {
			if rel != "" && !seen.Contains(rel) {
				seen.Add(rel)
				out = append(out, rel)
			}
		}
	}
	add(top)
	// A submodule that is not checked out lists nothing, and an error here leaves the top-level list.
	if nested, err := gitmod.CommandContext(ctx, root, "ls-files", "-z", "--recurse-submodules").Output(); err == nil {
		add(nested)
	}
	return out, nil
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
