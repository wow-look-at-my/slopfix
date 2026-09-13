// Package treecomments answers where the comments are, from a real syntax tree.
//
// It is the substrate adapter a comment rule reads: a comment is a node whose
// type carries "comment", which is how every grammar spells it, so nothing here
// names a language. A marker inside a string literal is not a comment, and a
// comment following code on its line is included, both because the parser says.
package treecomments

import (
	"fmt"
	"os"
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
	// The bash grammar reads a hash-comment format: it recovers the comments.
	".yml":  bash.Language,
	".yaml": bash.Language,
	".toml": bash.Language,
	".conf": bash.Language,
	".zsh":  bash.Language,
}

// Supported reports whether a grammar parses a file of that name. It asks the
// table of extensions rather than grammarFor, because loading a grammar decodes
// its parse tables and the question here is only whether one is named.
func Supported(filename string) bool {
	_, ok := grammars[strings.ToLower(filepath.Ext(filename))]
	return ok
}

// grammarFor answers the grammar an extension names. named is false when no
// extension matches, which is a different answer from a named grammar whose
// parse tables are absent: the first falls back to bash, the second must not.
func grammarFor(filename string) (language *ts.Language, named bool) {
	load, ok := grammars[strings.ToLower(filepath.Ext(filename))]
	if !ok {
		return nil, false
	}
	return load(), true
}

// languageFor answers the grammar to read a file with. Naming a file IS the
// request, so an unknown extension falls back to bash.
//
// It answers nil when the grammar it wants has no parse tables, and says so.
// A caller then scans no comments in that file, which is a rule going quiet
// rather than a rule passing. Nothing else reports it, so this does.
//
// A named grammar without tables never falls back to bash.
func languageFor(filename string) *ts.Language {
	language, named := grammarFor(filename)
	if named {
		if language == nil {
			reportMissingGrammar(filename)
		}
		return language
	}
	if language := bash.Language(); language != nil {
		return language
	}
	reportMissingGrammar(filename)
	return nil
}

// reported holds the extensions already named, so one absent grammar prints
// once rather than once per file.
var reported sync.Map

// reportMissingGrammar names an absent grammar and what it costs, on stderr.
func reportMissingGrammar(filename string) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = "(no extension)"
	}
	if _, seen := reported.LoadOrStore(ext, true); seen {
		return
	}
	fmt.Fprintf(os.Stderr,
		"slopfix: no parse tables for %s, so comment rules do not read those files. Run: go generate ./grammars/...\n",
		ext)
}

// Comment is a comment's text and where the tree says it begins.
type Comment struct {
	Text   string
	Offset int
	// Line is where it starts, counting from the top of the file.
	Line int
	// Col is where it starts in that line: past the indent means it follows code.
	Col int
	// Lines is how many lines it spans.
	Lines int
}

// Run is a stack of comments on adjoining lines, sharing a left edge.
type Run []Comment

// Runs groups a file's comments into the paragraphs a rewrite acts on.
//
// A run breaks where the comments stop adjoining, where the left edge moves,
// and where a comment follows code.
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

// indentOf reports the column the line's leading non-blank byte sits at.
func indentOf(src string, c Comment) int {
	start := strings.LastIndexByte(src[:c.Offset], '\n') + 1
	return len(src[start:c.Offset]) - len(strings.TrimLeft(src[start:c.Offset], " \t"))
}

// Extract returns every comment in the source, in source order.
//
// A syntax error yields comments anyway: tree-sitter recovers around it, so a
// rule still answers on a file mid-edit.
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
	return dropCgoPreamble(root, src, out)
}

// dropCgoPreamble removes the comment group cgo reads as C source.
func dropCgoPreamble(root ts.Node, src string, comments []Comment) []Comment {
	importAt := cgoImport(root, src)
	if importAt < 0 {
		return comments
	}
	// Upward from the import, because the group is found by what it adjoins.
	next, cut := importAt, -1
	for i := len(comments) - 1; i >= 0; i-- {
		c := comments[i]
		if c.Offset >= next {
			continue
		}
		if !onlyBlanksBetween(src, c.Offset+len(c.Text), next) {
			break
		}
		cut, next = i, c.Offset
	}
	if cut < 0 {
		return comments
	}
	// The group is adjoining, so it ends where the comments reach the import.
	end := cut
	for end < len(comments) && comments[end].Offset < importAt {
		end++
	}
	return append(comments[:cut:cut], comments[end:]...)
}

// cgoImport reports where a standalone `import "C"` starts, and a negative when
// the file has none. A grouped import carries no preamble, which is cgo's rule.
func cgoImport(root ts.Node, src string) int {
	count := root.NamedChildCount()
	for i := uint32(0); i < count; i++ {
		child := root.NamedChild(i)
		if child.IsNull() || child.Type() != "import_declaration" {
			continue
		}
		start, end := int(child.StartByte()), int(child.EndByte())
		if start < 0 || end > len(src) || start >= end {
			continue
		}
		if strings.Join(strings.Fields(src[start:end]), " ") == `import "C"` {
			return start
		}
	}
	return -1
}

// onlyBlanksBetween reports a gap carrying no blank line, which is where the
// preamble ends: a comment above a blank line is ordinary prose again.
func onlyBlanksBetween(src string, from, to int) bool {
	if from > to || to > len(src) {
		return false
	}
	return strings.TrimSpace(src[from:to]) == "" && strings.Count(src[from:to], "\n") <= 1
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
