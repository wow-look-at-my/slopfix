// shell.go checks the bash grammar's comments against a real bash parser.

package treecomments

import (
	"path/filepath"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"mvdan.cc/sh/v3/syntax"
)

// shellExts are the extensions that hold bash.
var shellExts = set.Of[string](".sh", ".bash")

// isShell answers whether bash reads this file.
func isShell(filename string) bool {
	return shellExts.Contains(strings.ToLower(filepath.Ext(filename)))
}

// shellComments keeps the comments that bash reads as comments. Tree-sitter
// recovery can lose a quote and read code after a `#` in a string as a comment.
func shellComments(src string, comments []Comment) []Comment {
	file, err := syntax.NewParser(syntax.KeepComments(true), syntax.Variant(syntax.LangBash)).
		Parse(strings.NewReader(src), "")
	if err != nil {
		// Without a parse, only a `#` that opens its line is safe to read as a comment.
		return wholeLineOnly(src, comments)
	}
	hashes := set.New[int]()
	syntax.Walk(file, func(n syntax.Node) bool {
		if c, ok := n.(*syntax.Comment); ok {
			hashes.Add(int(c.Hash.Offset()))
		}
		return true
	})
	kept := comments[:0]
	for _, c := range comments {
		if hashes.Contains(c.Offset) {
			kept = append(kept, c)
		}
	}
	return kept
}
