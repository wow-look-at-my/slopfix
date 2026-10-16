package commentfix

import (
	"fmt"
	"os"

	"github.com/wow-look-at-my/slopfix/gitread"
)

// withoutIgnored drops each path git ignores, such as built output. Git never
// ignores a tracked file, so every tracked file stays. Outside a work tree
// there are no ignore rules, and then every path stays.
func withoutIgnored(root string, paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	repo, err := gitread.OpenWorkTree(root)
	if err != nil {
		lostRead("ignored", err)
		return paths
	}
	if repo == nil {
		return paths
	}
	idx, err := repo.Index()
	if err != nil {
		lostRead("ignored", err)
		return paths
	}
	tracked := idx.Tracked()
	ignore := repo.Ignore()
	var kept []string
	for _, path := range paths {
		rel, ok := repo.Rel(path)
		if ok {
			if _, isTracked := tracked[rel]; !isTracked && ignore.Ignored(rel, false) {
				continue
			}
		}
		kept = append(kept, path)
	}
	return kept
}

// withoutVendored drops each path .gitattributes marks with any of
// BorrowedAttributes. Such text has another author, and no rule reads or
// rewrites it. Outside a work tree there are no attributes, and then every
// path stays.
func withoutVendored(root string, paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	repo, err := gitread.OpenWorkTree(root)
	if err != nil {
		lostRead("vendored", err)
		return paths
	}
	if repo == nil {
		return paths
	}
	attrs := repo.Attributes()
	var kept []string
	for _, path := range paths {
		if rel, ok := repo.Rel(path); ok && Borrowed(attrs, rel) {
			continue
		}
		kept = append(kept, path)
	}
	return kept
}

// Borrowed reports whether any of BorrowedAttributes is set on a path
// relative to the work tree.
func Borrowed(attrs *gitread.Attributes, rel string) bool {
	for _, name := range BorrowedAttributes {
		if AttributeSet(attrs.Value(rel, name)) {
			return true
		}
	}
	return false
}

// lostRead reports a repository read that failed inside a work tree. The walk
// then judges the paths that read would skip, and the reader must see why.
func lostRead(skip string, err error) {
	fmt.Fprintf(os.Stderr, "slopfix: cannot list the %s paths, so the walk judges them: %v\n", skip, err)
}

// BorrowedAttributes are the .gitattributes that mark a path as another author's: a vendored project.
var BorrowedAttributes = []string{"linguist-vendored", "linguist-generated"}

// AttributeSet reports a git attribute value that turns the attribute on.
func AttributeSet(value string) bool { return gitread.AttributeSet(value) }
