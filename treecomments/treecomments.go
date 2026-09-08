// Package treecomments answers where the comments are, from a real syntax tree.
//
// It is the substrate adapter a comment rule reads: a comment is a node whose
// type carries "comment", which is how every grammar spells it, so nothing here
// names a language. A marker inside a string literal is not a comment, and a
// comment following code on its line is included, both because the parser says.
package treecomments

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
	// A hash-comment format is read by the bash grammar: what it recovers from
	// a file that is not a script is the comment lines, which is all a comment
	// rule asks of it.
	".yml":  bash.Language,
	".yaml": bash.Language,
	".toml": bash.Language,
	".conf": bash.Language,
	".zsh":  bash.Language,
}

// Supported reports whether a grammar parses a file of that name. A walk reads
// it before opening a file, so a tree it has no grammar for is skipped rather
// than guessed at.
func Supported(filename string) bool { return grammarFor(filename) != nil }

// grammarFor answers the grammar an extension names, and nil when none does.
func grammarFor(filename string) *ts.Language {
	if load, ok := grammars[strings.ToLower(filepath.Ext(filename))]; ok {
		return load()
	}
	return nil
}

// languageFor answers the grammar to read a file with. Naming a file IS the
// request, so one whose extension names no grammar is read by the bash grammar
// rather than skipped: a Dockerfile, a Makefile and a dotfile all carry hash
// comments, and that is what the caller asked about.
func languageFor(filename string) *ts.Language {
	if language := grammarFor(filename); language != nil {
		return language
	}
	return bash.Language()
}

// Comment is a comment's text and where it begins in the source.
type Comment struct {
	Text   string
	Offset int
}

// Extract returns every comment in the source, in source order.
//
// A file no grammar covers yields nothing. A file with a syntax error does
// not: tree-sitter recovers around the error, so the comments it does find sit
// where it says they do, and a rule answers on a file mid-edit.
func Extract(filename, src string) []Comment {
	language := languageFor(filename)
	if language == nil {
		return nil
	}
	parser := ts.NewParser()
	if !parser.SetLanguage(language) {
		return nil
	}
	tree := parser.ParseString(nil, []byte(src))
	if tree == nil {
		return nil
	}
	root := tree.RootNode()
	if root.IsNull() {
		return nil
	}
	var out []Comment
	collect(root, src, &out)
	return out
}

// collect walks the tree and gathers every comment node under it.
func collect(node ts.Node, src string, out *[]Comment) {
	count := node.NamedChildCount()
	for i := uint32(0); i < count; i++ {
		child := node.NamedChild(i)
		if child.IsNull() {
			continue
		}
		if strings.Contains(child.Type(), "comment") {
			start, end := int(child.StartByte()), int(child.EndByte())
			if start >= 0 && end <= len(src) && start < end {
				*out = append(*out, Comment{Text: src[start:end], Offset: start})
			}
			continue
		}
		collect(child, src, out)
	}
}
