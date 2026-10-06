package commentfix

import (
	"github.com/wow-look-at-my/slopfix/gitread"
)

// withoutIgnored drops each path git ignores, such as built output. A tracked
// file is never ignored, so every tracked file stays. Outside a work tree
// there are no ignore rules, and then every path stays.
func withoutIgnored(root string, paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	repo, err := gitread.OpenWorkTree(root)
	if err != nil || repo == nil {
		return paths
	}
	ignore := repo.Ignore()
	tracked := map[string]gitread.IndexEntry{}
	if idx, err := repo.Index(); err == nil {
		tracked = idx.Tracked()
	}
	var kept []string
	for _, path := range paths {
		rel, ok := repoRel(repo, path)
		if !ok {
			kept = append(kept, path)
			continue
		}
		if _, isTracked := tracked[rel]; isTracked {
			kept = append(kept, path)
			continue
		}
		if ignore.Ignored(rel, false) {
			continue
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
	if err != nil || repo == nil {
		return paths
	}
	var kept []string
	for _, path := range paths {
		rel, ok := repoRel(repo, path)
		if ok && vendoredIn(repo, rel) {
			continue
		}
		kept = append(kept, path)
	}
	return kept
}

// vendoredIn reports whether any borrowed attribute is set on a repo path.
func vendoredIn(repo *gitread.Repo, rel string) bool {
	for _, name := range BorrowedAttributes {
		if gitread.AttributeSet(repo.Attr(rel, name)) {
			return true
		}
	}
	return false
}

// repoRel answers path relative to the repository's work tree, in slash form.
func repoRel(repo *gitread.Repo, path string) (string, bool) {
	return repo.Rel(path)
}

// BorrowedAttributes are the .gitattributes that mark a path as another author's: a vendored project.
var BorrowedAttributes = []string{"linguist-vendored", "linguist-generated"}

// AttributeSet reports a git attribute value that turns the attribute on.
func AttributeSet(value string) bool { return gitread.AttributeSet(value) }
