// tree.go sweeps a whole tree with every rule, which is what a build calls.
//
// commentfix.FixTree reaches the comment rules alone. A build that ran it
// repaired a comment's length and left the ste finding beside it. A caller
// that wants a single rule family still has the narrower sweep. This is for
// the caller that wants what `slopfix file fix` would do.
package slopfix

import (
	"os"
	"runtime"
	"slices"
	"sync"

	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/trace"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// TreeRepair is what a whole-tree run did.
type TreeRepair struct {
	// Read is how many files the walk opened.
	Read int
	// Repaired names each file rewritten.
	Repaired []string
	// Removed quotes what a repair deleted, and is the only record of it.
	Removed []commentfix.Removal
	// Findings carry what no repair covered.
	Findings []TreeFinding
	// Kept carries the tombstones no whole-line deletion resolves.
	Kept []TreeTombstone
	// Unmet names each fixture whose slopfix-expect annotations did not hold.
	Unmet []*UnmetError
}

// TreeFinding is an ste finding and the file it was found in.
type TreeFinding struct {
	Path string
	ste.Finding
}

// TreeTombstone is a tombstone a repair could not strip, and its file.
type TreeTombstone struct {
	Path string
	tombstones.Hit
}

// Reads reports whether any rule reads a file of that name.
func Reads(path string) bool {
	return IsDocument(path) || commentfix.Supported(path) || workflow.Judges(path)
}

// FixTree repairs every file under root with every rule and reports what it did.
func FixTree(root string) TreeRepair { return FixTreeWith(root, Request{}) }

// CheckTree reports what every rule makes of every file under root.
func CheckTree(root string) TreeRepair { return treeRun(root, Request{}, false) }

// FixTreeWith is FixTree under the caller's own rule selection.
func FixTreeWith(root string, req Request) TreeRepair { return treeRun(root, req, true) }

// CheckTreeWith is CheckTree under the caller's own rule selection.
func CheckTreeWith(root string, req Request) TreeRepair { return treeRun(root, req, false) }

// wantsRepo reports whether the caller's selection reaches the repository rules.
func wantsRepo(req Request) bool {
	return len(req.Rules) == 0 || slices.Contains(req.Rules, RuleRepo)
}

// keepsOf answers the caller's ID selection as a test, where naming none keeps all.
func keepsOf(req Request) func(string) bool {
	return func(id string) bool { return len(req.IDs) == 0 || slices.Contains(req.IDs, id) }
}

// treeRun walks root and answers what every rule made of each file, writing
// each repair back when writing is asked for.
func treeRun(root string, req Request, writing bool) TreeRepair {
	defer trace.Phase("slopfix/fixtree")()

	var out TreeRepair
	if wantsRepo(req) && isRepoRoot(root) {
		SetPhase("repository rules", 0)
		findings, changed, err := repoRun(root, keepsOf(req), writing, writableIn(req.Fork))
		if err != nil {
			findings = append(findings, repoFinding(root, IDBudget, "the repository rules could not read the tree", err.Error()))
		}
		out.Findings = append(out.Findings, findings...)
		out.Repaired = append(out.Repaired, changed...)
	}
	SetPhase("list the files", 0)
	paths := commentfix.TreeFilesMatching(root, Reads)
	SetPhase("index the names", 0)
	// A walk asks the referent check about far more names than a probe for each can answer quickly.
	tombstones.PrimeIndex(root)
	SetPhase("judge the files", len(paths))
	repairs := make([]*Repair, len(paths))
	// Each file is independent, so workers judge them at once and the merge below keeps the walk's order.
	work := make(chan int)
	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				repairs[i] = judgeFile(paths[i], req, writing)
				stepDone()
			}
		}()
	}
	for i := range paths {
		work <- i
	}
	close(work)
	wg.Wait()

	for i, path := range paths {
		repair := repairs[i]
		if repair == nil {
			continue
		}
		out.Read++
		if len(repair.Unmet) > 0 {
			out.Unmet = append(out.Unmet, &UnmetError{Path: path, Unmet: repair.Unmet})
		}
		if writing && repair.Changed && commentfix.WriteFile(path, repair.Text) == nil {
			out.Repaired = append(out.Repaired, path)
		}
		for _, text := range repair.Removed {
			out.Removed = append(out.Removed, commentfix.Removal{Path: path, Text: text})
		}
		for _, finding := range repair.Findings {
			out.Findings = append(out.Findings, TreeFinding{Path: path, Finding: finding})
		}
		for _, kept := range repair.Kept {
			out.Kept = append(out.Kept, TreeTombstone{Path: path, Hit: kept})
		}
	}
	return out
}

// judgeFile runs every selected rule on one file. It answers nil for a file it
// cannot read, and for a file of a fork that the fork never touched.
func judgeFile(path string, req Request, writing bool) *Repair {
	if req.Fork != nil {
		scope := req.Fork.Scope(path)
		if scope.Empty() {
			return nil
		}
		req.Owned = scope
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	req.Path, req.Content = path, string(src)
	run := Report
	if writing {
		run = Fix
	}
	repair := run(req)
	return &repair
}
