// Package code is the parse every rule reads source through.
//
// Nothing here names a language beyond the grammar registry. A comment is a
// node whose type carries "comment", which is how every grammar spells it, so
// a rule asks the tree rather than a table of marker bytes.
package code

import (
	"path/filepath"
	"strings"

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

// Parse returns the root of the syntax tree.
//
// ok is false when no grammar parses the file, or when the parse carries an
// error. A file mid-edit is the common case for a hook, and half a tree reads
// code as prose.
func Parse(filename, src string) (root ts.Node, ok bool) {
	language := LanguageFor(filename)
	if language == nil {
		return ts.Node{}, false
	}
	return ParseWith(language, src)
}

// ParseWith is Parse for a caller that already resolved the grammar.
func ParseWith(language *ts.Language, src string) (root ts.Node, ok bool) {
	parser := ts.NewParser()
	if !parser.SetLanguage(language) {
		return ts.Node{}, false
	}
	tree := parser.ParseString(nil, []byte(src))
	if tree == nil {
		return ts.Node{}, false
	}
	root = tree.RootNode()
	if root.IsNull() || root.HasError() {
		return ts.Node{}, false
	}
	return root, true
}

// IsComment reports a node every grammar spells as a comment.
func IsComment(node ts.Node) bool {
	return !node.IsNull() && strings.Contains(node.Type(), "comment")
}

// StartsComment reports whether a trimmed line opens a comment.
func StartsComment(trimmed string) bool {
	for _, marker := range []string{"///", "//", "/*", "*", "#"} {
		if strings.HasPrefix(trimmed, marker) {
			return true
		}
	}
	return false
}

// Lines splits source the way every span here counts it.
func Lines(src string) []string { return strings.Split(src, "\n") }
