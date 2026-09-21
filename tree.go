// tree.go sweeps a whole tree with every rule, which is what a build calls.
//
// commentfix.FixTree reaches the comment rules alone, so a build running it
// repaired a comment's length and left the ste finding beside it for a reviewer
// to hit. A caller that wants a single rule family still has the narrower
// sweep; this is for the caller that wants what `slopfix file fix` would do.
package slopfix

import (
	"os"

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
}

// TreeFinding is an ste finding and the file it was found in. A finding knows
// its line and not its file, because a single call reads a single file.
type TreeFinding struct {
	Path string
	ste.Finding
}

// TreeTombstone is a tombstone a repair could not strip, and its file.
type TreeTombstone struct {
	Path string
	tombstones.Hit
}

// Reads reports whether any rule reads a file of that name. It is the same
// question `slopfix file` asks of a directory argument, so a build and the
// command line select the same files.
func Reads(path string) bool {
	return IsDocument(path) || commentfix.Supported(path) || workflow.Judges(path)
}

// FixTree repairs every file under root with every rule and reports what it
// did. A file is written only when a repair changed it.
func FixTree(root string) TreeRepair { return FixTreeWith(root, Request{}) }

// CheckTree reports what every rule makes of every file under root and writes
// nothing, so the report a build prints names what --fix would have done.
func CheckTree(root string) TreeRepair { return treeRun(root, Request{}, false) }

// FixTreeWith is FixTree under the caller's own rule selection.
func FixTreeWith(root string, req Request) TreeRepair { return treeRun(root, req, true) }

// CheckTreeWith is CheckTree under the caller's own rule selection.
func CheckTreeWith(root string, req Request) TreeRepair { return treeRun(root, req, false) }

// treeRun walks root and answers what every rule made of each file, writing
// each repair back when writing is asked for.
func treeRun(root string, req Request, writing bool) TreeRepair {
	defer trace.Phase("slopfix/fixtree")()

	var out TreeRepair
	for _, path := range commentfix.TreeFilesMatching(root, Reads) {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out.Read++

		req.Path, req.Content = path, string(src)
		repair := Fix(req)
		if writing && repair.Changed && os.WriteFile(path, []byte(repair.Text), 0o644) == nil {
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
