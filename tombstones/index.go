package tombstones

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitmod"
)

// indexTimeout bounds the read of the tree.
const indexTimeout = 5 * time.Minute

// indexFileCap is the largest tracked file the index reads. A larger one is data.
const indexFileCap = 4 << 20

// symbolIndex holds every identifier-shaped word the tracked files contain, with the count of files that hold it.
type symbolIndex struct {
	once  sync.Once
	files map[string]int
	ok    bool

	pathOnce sync.Once
	paths    set.Set[string]
	bases    set.Set[string]
	pathsOK  bool

	blindOnce sync.Once
	blind     bool
}

// missingDependency reports a submodule with no checkout under root. A name the
// repository takes from it, such as an API that a registry submodule defines,
// is in no file a probe can read. The probe then cannot answer.
func (ix *symbolIndex) missingDependency(root string) bool {
	ix.blindOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), indexTimeout)
		defer cancel()
		// With no git to list the gitlinks, no submodule is known, and the probe answers as it does without one.
		listed, err := gitmod.CommandContext(ctx, root, "ls-files", "-z", "--stage").Output()
		if err != nil {
			return
		}
		for _, entry := range strings.Split(string(listed), "\x00") {
			meta, rel, ok := strings.Cut(entry, "\t")
			if !ok || !strings.HasPrefix(meta, "160000 ") {
				continue
			}
			if children, err := os.ReadDir(filepath.Join(root, rel)); err != nil || len(children) == 0 {
				ix.blind = true
				return
			}
		}
	})
	return ix.blind
}

// listPaths reads the paths git lists under root, once.
func (ix *symbolIndex) listPaths(root string) {
	ix.pathOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), indexTimeout)
		defer cancel()
		listed, err := gitmod.CommandContext(ctx, root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
		if err != nil {
			return
		}
		ix.paths = set.New[string]()
		ix.bases = set.New[string]()
		for _, rel := range strings.Split(string(listed), "\x00") {
			for _, word := range identifierWords(rel) {
				ix.paths.Add(word)
			}
			if rel != "" {
				ix.bases.Add(filepath.Base(rel))
			}
		}
		ix.pathsOK = true
	})
}

// namesAFile reports a name that a path git lists under root spells, such as
// sig_cosmo_test for sig_cosmo_test.go. A name that ends in an underscore
// names the files that start with it.
func (ix *symbolIndex) namesAFile(root, name string) bool {
	ix.listPaths(root)
	if !ix.pathsOK {
		return false
	}
	if ix.paths.Contains(name) {
		return true
	}
	if !strings.HasSuffix(name, "_") {
		return false
	}
	for word := range ix.paths.All() {
		if strings.HasPrefix(word, name) {
			return true
		}
	}
	return false
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
// untracked unless git ignores them, and tracked in a submodule checkout. That
// is what ripgrep reads.
func (ix *symbolIndex) build(root string) {
	ctx, cancel := context.WithTimeout(context.Background(), indexTimeout)
	defer cancel()
	listed, err := gitmod.CommandContext(ctx, root, "ls-files", "-z", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return
	}
	// A failed submodule listing adds no file, and the index keeps the files of the tree itself.
	nested, _ := gitmod.CommandContext(ctx, root, "ls-files", "-z", "--cached", "--recurse-submodules").Output()
	files := map[string]int{}
	seen := set.New[string]()
	paths := append(strings.Split(string(listed)+"\x00"+string(nested), "\x00"), copiedSubmoduleFiles(ctx, root)...)
	for _, rel := range paths {
		if rel == "" || seen.Contains(rel) {
			continue
		}
		seen.Add(rel)
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

// copiedSubmoduleFiles lists the files under each submodule path that holds
// files but no checkout. A build that copies a dependency in place of a
// checkout leaves such a path, and git lists none of its files.
func copiedSubmoduleFiles(ctx context.Context, root string) []string {
	staged, err := gitmod.CommandContext(ctx, root, "ls-files", "-z", "--stage").Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, entry := range strings.Split(string(staged), "\x00") {
		meta, rel, ok := strings.Cut(entry, "\t")
		if !ok || !strings.HasPrefix(meta, "160000 ") {
			continue
		}
		if _, err := os.Lstat(filepath.Join(root, rel, ".git")); err == nil {
			continue
		}
		_ = filepath.WalkDir(filepath.Join(root, rel), func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil || !d.Type().IsRegular() {
				return nil
			}
			if name, err := filepath.Rel(root, path); err == nil {
				files = append(files, name)
			}
			return nil
		})
	}
	return files
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
