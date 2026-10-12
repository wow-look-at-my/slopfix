package treecomments

import (
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// hashNames are the file names, extensions apart, whose comments open on `#`.
var hashNames = set.Of[string]("dockerfile",
	"containerfile",
	"makefile",
	"justfile",
	"cmakelists.txt")

// hashExts are the extensions whose comments open on `#` and that no grammar
// in the table names. The bash grammar recovers their comments.
var hashExts = set.Of[string](".ini",
	".cfg",
	".dockerfile",
	".mk",
	".dats",
	".cmake")

// HashComments answers whether this file's comments open on a `#`.
func HashComments(filename string) bool {
	base := strings.ToLower(filepath.Base(filename))
	if hashNames.Contains(base) {
		return true
	}
	return hashExts.Contains(strings.ToLower(filepath.Ext(base)))
}
