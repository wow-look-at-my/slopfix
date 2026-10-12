// hashcomments.go names the files whose comments open on a `#`.
//
// A Dockerfile and a Makefile carry prose the gate reads, and no grammar is
// keyed to their names. treecomments owns the membership, because its bash
// fallback is what reads that shape. What is here is the walk: which names it
// opens rather than skips.
package commentfix

import "github.com/wow-look-at-my/slopfix/treecomments"

// readsHash answers whether this file's comments open on a `#`.
func readsHash(filename string) bool {
	return treecomments.HashComments(filename)
}
