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

// Comment is a comment's text and where the tree says it begins.
type Comment struct {
	Text   string
	Offset int
	// Line is where it starts, counting from the top of the file.
	Line int
	// Col is the byte it starts at within that line, counting from the left.
	// A comment with code before it on the line has a column past the indent,
	// and that is what tells the two apart.
	Col int
	// Lines is how many lines it spans: more than a single line for a block
	// comment, and always a single line for the line-comment form.
	Lines int
}

// Run is a stack of comments on adjoining lines, sharing a left edge. It is the
// paragraph a rewrite acts on: a sentence wraps across the lines of a run, so a
// repair that reads a line alone cuts sentences in half.
type Run []Comment

// Runs groups a file's comments into the paragraphs a rewrite acts on.
//
// A run breaks where the tree says the comments stop adjoining, where the left
// edge moves, and where a comment follows code: none of those continues the
// sentence above it.
func Runs(filename, src string) []Run {
	var out []Run
	for _, c := range Extract(filename, src) {
		if n := len(out); n > 0 {
			last := out[n-1][len(out[n-1])-1]
			adjoins := c.Line == last.Line+last.Lines && c.Col == last.Col && c.Col == indentOf(src, c)
			if adjoins {
				out[n-1] = append(out[n-1], c)
				continue
			}
		}
		out = append(out, Run{c})
	}
	return out
}

// indentOf reports the column the line's first non-blank byte sits at, so a
// comment that follows code can be told from one that opens its line.
func indentOf(src string, c Comment) int {
	start := strings.LastIndexByte(src[:c.Offset], '\n') + 1
	return len(src[start:c.Offset]) - len(strings.TrimLeft(src[start:c.Offset], " \t"))
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
				*out = append(*out, Comment{
					Text:   src[start:end],
					Offset: start,
					Line:   int(child.StartPoint().Row) + 1,
					Col:    int(child.StartPoint().Column),
					Lines:  int(child.EndPoint().Row-child.StartPoint().Row) + 1,
				})
			}
			continue
		}
		collect(child, src, out)
	}
}
