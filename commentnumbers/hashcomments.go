// hashcomments.go names the files whose comments open on a `#`.
//
// A workflow, a Dockerfile and a Makefile all carry prose the gate reads, and
// no grammar here is keyed to their names. The comments themselves come from
// treecomments, whose bash fallback reads that shape. What is here is the
// membership: which names a walk opens rather than skips.
package commentnumbers

import (
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// hashNames are the file names, extensions apart, whose comments open on `#`.
var hashNames = set.Of[string]("dockerfile",
	"containerfile",
	"makefile",
	"justfile")

// hashExts are the extensions whose comments open on `#`.
var hashExts = set.Of[string](".yml",
	".yaml",
	".toml",
	".ini",
	".cfg",
	".conf",
	".dockerfile",
	".mk",
	".dats")

// readsHash answers whether this file's comments open on a `#`.
func readsHash(filename string) bool {
	base := strings.ToLower(filepath.Base(filename))
	if hashNames.Contains(base) {
		return true
	}
	return hashExts.Contains(strings.ToLower(filepath.Ext(base)))
}

