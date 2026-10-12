// Package code is the parse every rule reads source through.
//
// Nothing here names a language beyond the grammar registry. A comment is a
// node whose type carries "comment", which is how every grammar spells it. A
// rule asks the tree rather than a table of marker bytes.
package code

import (
	"path/filepath"
	"strings"
	"sync"

	ts "github.com/wow-look-at-my/go-tree-sitter"
	"github.com/wow-look-at-my/slopfix/grammars/bash"
	"github.com/wow-look-at-my/slopfix/grammars/clang"
	"github.com/wow-look-at-my/slopfix/grammars/cpp"
	"github.com/wow-look-at-my/slopfix/grammars/golang"
	"github.com/wow-look-at-my/slopfix/grammars/javascript"
	"github.com/wow-look-at-my/slopfix/grammars/rust"
	"github.com/wow-look-at-my/slopfix/grammars/tsx"
	"github.com/wow-look-at-my/slopfix/grammars/typescript"
)

// grammars maps a file extension to the grammar that parses it.
var grammars = map[string]func() *ts.Language{
	".go":   golang.Language,
	".c":    clang.Language,
	".h":    clang.Language,
	".cc":   cpp.Language,
	".cpp":  cpp.Language,
	".cxx":  cpp.Language,
	".hpp":  cpp.Language,
	".hh":   cpp.Language,
	".rs":   rust.Language,
	".sh":   bash.Language,
	".bash": bash.Language,
	".js":   javascript.Language,
	".jsx":  javascript.Language,
	".mjs":  javascript.Language,
	".cjs":  javascript.Language,
	".ts":   typescript.Language,
	".mts":  typescript.Language,
	".cts":  typescript.Language,
	".tsx":  tsx.Language,
}

// LanguageFor answers the grammar for a filename, and nil when none parses it.
func LanguageFor(filename string) *ts.Language {
	if load, ok := grammars[strings.ToLower(filepath.Ext(filename))]; ok {
		return load()
	}
	return nil
}

// Parsed reports whether a grammar reads this file rather than skipping it.
func Parsed(filename string) bool { return LanguageFor(filename) != nil }

// Extensions is every extension a grammar claims, for a caller asking whether
// a file can be parsed at all.
func Extensions() []string {
	out := make([]string, 0, len(grammars))
	for ext := range grammars {
		out = append(out, ext)
	}
	return out
}

// Parse returns the root of the syntax tree.
func Parse(filename, src string) (root ts.Node, ok bool) {
	language := LanguageFor(filename)
	if language == nil {
		return ts.Node{}, false
	}
	return ParseWith(language, src)
}

// ParseWith is Parse for a caller that already resolved the grammar.
func ParseWith(language *ts.Language, src string) (root ts.Node, ok bool) {
	return whole(Tree(language, src))
}

// whole answers the root of a tree that parsed with no error.
func whole(tree *ts.Tree) (root ts.Node, ok bool) {
	if tree == nil {
		return ts.Node{}, false
	}
	root = tree.RootNode()
	if root.IsNull() || root.HasError() {
		return ts.Node{}, false
	}
	return root, true
}

// Tree answers the syntax tree of src, errors and all, or nil when the grammar
// does not load. Each rule reads the same file through its own parse, so the
// latest trees are kept. A tree is only read, never edited, so rules share it.
func Tree(language *ts.Language, src string) *ts.Tree {
	key := treeKey{language, src}
	trees.Lock()
	tree, ok := trees.byKey[key]
	trees.Unlock()
	if ok {
		return tree
	}
	tree = parseTree(language, src)
	trees.Lock()
	defer trees.Unlock()
	if _, ok := trees.byKey[key]; ok {
		return tree
	}
	if len(trees.order) == treeCap {
		delete(trees.byKey, trees.order[0])
		trees.order = trees.order[1:]
	}
	trees.order = append(trees.order, key)
	trees.byKey[key] = tree
	return tree
}

// treeCap is how many trees Tree keeps: one file for each worker of a tree walk.
const treeCap = 128

type treeKey struct {
	language *ts.Language
	src      string
}

var trees = struct {
	sync.Mutex
	byKey map[treeKey]*ts.Tree
	order []treeKey
}{byKey: map[treeKey]*ts.Tree{}}

// parseTree runs the grammar over src.
func parseTree(language *ts.Language, src string) *ts.Tree {
	parser := ts.NewParser()
	if !parser.SetLanguage(language) {
		return nil
	}
	return parser.ParseString(nil, []byte(src))
}

// IsComment reports a node every grammar spells as a comment.
func IsComment(node ts.Node) bool {
	return !node.IsNull() && strings.Contains(node.Type(), "comment")
}

// IsInterpreter reports the line a script opens on.
func IsInterpreter(node ts.Node, src string) bool {
	return IsComment(node) && node.StartByte() == 0 && node.EndPoint().Row == 0 && strings.HasPrefix(src, "#!")
}

// Lines splits source the way every span here counts it.
func Lines(src string) []string { return strings.Split(src, "\n") }
