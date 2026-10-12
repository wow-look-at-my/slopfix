package forkscope

import (
	"fmt"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// tagBase answers the newest commit of HEAD's history that an upstream tag
// names, or "" when HEAD contains none. A fork that merges upstream releases
// carries that tag.
func tagBase(top string, tags []string) (string, error) {
	if err := deepen(top); err != nil {
		return "", err
	}
	commits := set.Of(tags...)
	revs, err := gitIn(top, "rev-list", "HEAD")
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	for rev := range strings.SplitSeq(revs, "\n") {
		if commits.Contains(rev) {
			return rev, nil
		}
	}
	return "", nil
}

// snapshotBase answers the commit of the upstream's default branch whose tree
// differs from HEAD's in the fewest paths. The newer commit wins a tie.
//
// A fork that squashes each upstream sync shares no history with the upstream
// after the first sync. The merge base then predates every later sync, and the
// diff from it counts upstream's work as the fork's. The closest tree is the
// snapshot the last sync brought in.
//
// The candidates are the upstream commits after the merge base, and the merge
// base itself. With no merge base, every commit of the branch is a candidate.
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
	args := []string{"rev-list", "--first-parent", rec.Tip}
	mergeBase := ""
	if out, err := gitIn(top, "merge-base", "HEAD", rec.Tip); err == nil {
		mergeBase = strings.TrimSpace(out)
		args = append(args, "^"+mergeBase)
	}
	out, err := gitIn(top, args...)
	if err != nil {
		return "", fmt.Errorf("fork scope: list the commits of %s: %w", rec.Upstream, err)
	}
	candidates := strings.Fields(out)
	if mergeBase != "" {
		candidates = append(candidates, mergeBase)
	}
	best, fewest := "", -1
	for _, commit := range candidates {
		names, err := gitIn(top, "diff", "--name-only", "--no-renames", commit, "HEAD", "--")
		if err != nil {
			return "", fmt.Errorf("fork scope: compare HEAD with %s of %s: %w", commit, rec.Upstream, err)
		}
		if n := strings.Count(names, "\n"); fewest < 0 || n < fewest {
			best, fewest = commit, n
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
	out, err := gitIn(top, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", fmt.Errorf("fork scope: resolve the default branch of %s: %w", upstream, err)
	}
	return strings.TrimSpace(out), nil
}

// hasCommit reports whether commit is in top's object store.
func hasCommit(top, commit string) bool {
	_, err := gitIn(top, "cat-file", "-e", commit+"^{commit}")
	return err == nil
}
