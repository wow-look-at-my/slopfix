package forkscope

import (
	"fmt"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/gitread"
)

// tagBase answers the newest commit of HEAD's history that an upstream tag
// names, or "" when HEAD contains none. A fork that merges upstream releases
// carries that tag.
func tagBase(g *gitread.Repo, tags []string) (string, error) {
	if err := deepen(g); err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return "", nil
	}
	wanted := set.Of(tags...)
	head, err := g.Head()
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	revs, err := g.RevList(head)
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	for _, rev := range revs {
		if wanted.Contains(rev.String()) {
			return rev.String(), nil
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
func snapshotBase(g *gitread.Repo, rec *record) (string, error) {
	if rec.Tip == "" || !hasCommit(g, rec.Tip) {
		tip, err := fetchUpstreamHead(g, rec.Upstream)
		if err != nil {
			return "", err
		}
		rec.Tip = tip
		if err := store(g, *rec); err != nil {
			return "", err
		}
	}
	tip, err := gitread.ParseOID(rec.Tip)
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	head, err := g.Head()
	if err != nil {
		return "", fmt.Errorf("fork scope: %w", err)
	}
	candidates, err := g.FirstParent(tip)
	if err != nil {
		return "", fmt.Errorf("fork scope: list the commits of %s: %w", rec.Upstream, err)
	}
	var list []gitread.OID
	list = append(list, candidates...)
	if base, ok, err := g.MergeBase(head, tip); err == nil && ok {
		list = append(list, base)
	}
	best := gitread.OID{}
	fewest := -1
	for _, commit := range list {
		names, err := g.ChangedPaths(commit, head)
		if err != nil {
			return "", fmt.Errorf("fork scope: compare HEAD with %s of %s: %w", commit, rec.Upstream, err)
		}
		if n := len(names); fewest < 0 || n < fewest {
			best, fewest = commit, n
		}
	}
	if best.IsZero() {
		return "", fmt.Errorf("fork scope: the default branch of %s has no commits", rec.Upstream)
	}
	return best.String(), nil
}

// fetchUpstreamHead reads the default branch of upstream and answers its commit.
func fetchUpstreamHead(g *gitread.Repo, upstream string) (string, error) {
	commit, remote, err := gitread.FetchRef(upstream, "HEAD", g)
	if err != nil {
		return "", fmt.Errorf("fork scope: fetch the default branch of %s: %w", upstream, err)
	}
	g.AddAlternate(remote)
	return commit.String(), nil
}

// hasCommit reports whether commit is in the object store.
func hasCommit(g *gitread.Repo, commit string) bool {
	oid, err := gitread.ParseOID(commit)
	if err != nil {
		return false
	}
	_, err = g.PeelCommit(oid)
	return err == nil
}
