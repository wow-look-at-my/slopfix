// hashcomments.go reads the comments of a file no grammar here parses.
//
// A workflow, a Dockerfile and a Makefile all carry prose the gate reads, and
// none of them has a tree-sitter grammar in this repository. Their comment
// syntax is the same shape in every case: a `#` runs to the end of its line.
//
// This is not a parser and does not pretend to be. A `#` inside a quoted string
// reads as a comment here, which costs a false finding on a line that is
// already unusual. The alternative was reading none of these files at all.
package commentnumbers

import (
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/commentlength"
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

// extract returns a file's comments, from its grammar where it has one and
// from the hash reader otherwise.
func extract(filename, src string) []commentlength.Comment {
	if commentlength.Parsed(filename) {
		return commentlength.Comments(filename, src)
	}
	if !readsHash(filename) {
		return nil
	}
	return hashComments(src)
}

// hashComments returns each `#` run to the end of its line, with the offset
// the caller counts from. A shebang is not prose, so it is skipped.
func hashComments(src string) []commentlength.Comment {
	var out []commentlength.Comment
	at := 0
	for i, line := range strings.Split(src, "\n") {
		if hash := strings.IndexByte(line, '#'); hash >= 0 {
			if !(i == 0 && strings.HasPrefix(line, "#!")) {
				out = append(out, commentlength.Comment{Text: line[hash:], Offset: at + hash})
			}
		}
		at += len(line) + 1
	}
	return out
}
