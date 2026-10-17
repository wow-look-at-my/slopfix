package forkscope

import (
	"fmt"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitread"
)

// tagBase answers the newest commit of HEAD's history that an upstream tag
// names, or "" when HEAD contains none. A fork that merges upstream releases
// carries that tag.
func tagBase(top string, tags []string) (string, error) {
	if err := deepen(top); err != nil {
		return "", err
	}
	commits := set.New[gitread.OID]()
	for _, tag := range tags {
		if oid, err := gitread.ParseOID(tag); err == nil {
			commits.Add(oid)
		}
	}
	head, repo, err := resolve(top, "HEAD")
	if err != nil {
		return "", err
	}
	found := ""
	err = repo.Walk([]gitread.OID{head}, func(c *gitread.Commit) bool {
		if commits.Contains(c.OID) {
			found = c.OID.String()
			return false
		}
		return true
	})
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	return found, nil
}

// snapshotBase answers the commit of the upstream's default branch whose tree
// differs from HEAD's in the fewest paths. The newer commit wins a tie.
//
// A fork that squashes each upstream sync shares no history with the upstream
// after the first sync. The merge base then predates every later sync, and the
// diff from it counts upstream's work as the fork's. The closest tree is the
// snapshot the last sync brought in.
//
// With no merge base, every commit of the branch is a candidate.
func snapshotBase(top string, rec *record) (string, error) {
	if rec.Tip == "" || !hasCommit(top, rec.Tip) {
		tip, err := fetchUpstreamHead(top, rec.Upstream)
		if err != nil {
			return "", err
		}
		rec.Tip = tip
		if err := store(top, *rec); err != nil {
			return "", err
		}
	}
	tip, repo, err := resolve(top, rec.Tip)
	if err != nil {
		return "", err
	}
	head, _, err := resolve(top, "HEAD")
	if err != nil {
		return "", err
	}
	var exclude []gitread.OID
	mergeBase, hasBase, err := repo.MergeBase(head, tip)
	if err != nil {
		return "", fmt.Errorf("fork scope: list the commits of %s: %w", rec.Upstream, err)
	}
	if hasBase {
		exclude = append(exclude, mergeBase)
	}
	candidates, err := repo.FirstParentExcluding(tip, exclude)
	if err != nil {
		return "", fmt.Errorf("fork scope: list the commits of %s: %w", rec.Upstream, err)
	}
	if hasBase {
		candidates = append(candidates, mergeBase)
	}
	best, fewest := "", -1
	for _, commit := range candidates {
		n := 0
		if err := repo.DiffTrees(commit, head, func(string, *gitread.TreeEntry, *gitread.TreeEntry) { n++ }); err != nil {
			return "", fmt.Errorf("fork scope: compare HEAD with %s of %s: %w", commit, rec.Upstream, err)
		}
		if fewest < 0 || n < fewest {
			best, fewest = commit.String(), n
		}
	}
	if best == "" {
		return "", fmt.Errorf("fork scope: the default branch of %s has no commits", rec.Upstream)
	}
	return best, nil
}

// fetchUpstreamHead fetches the default branch of upstream into top, with its
// history and without blobs, and answers its commit.
func fetchUpstreamHead(top, upstream string) (string, error) {
	if _, err := gitIn(top, "fetch", "--quiet", "--filter=blob:none", "--no-tags", upstream, "HEAD"); err != nil {
		return "", fmt.Errorf("fork scope: fetch the default branch of %s: %w", upstream, err)
	}
	commit, _, err := resolve(top, "FETCH_HEAD")
	if err != nil {
		return "", fmt.Errorf("fork scope: resolve the default branch of %s: %w", upstream, err)
	}
	return commit.String(), nil
}

// hasCommit reports whether commit is in top's object store.
func hasCommit(top, commit string) bool {
	_, _, err := resolve(top, commit)
	return err == nil
}
