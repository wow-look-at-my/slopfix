// Package treecomments answers where the comments are, from a real syntax tree.
//
// It is the substrate adapter a comment rule reads. A comment is a node whose
// type carries "comment", which is how every grammar spells it, so nothing here
// names a language. A marker inside a string literal is not a comment, and a
// comment following code on its line is included, both because the parser says.
package treecomments

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	"github.com/wow-look-at-my/slopfix/trace"
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

// Supported reports whether a grammar parses a file of that name.
func Supported(filename string) bool {
	_, ok := grammars[strings.ToLower(filepath.Ext(filename))]
	return ok
}

// grammarFor answers the grammar an extension names. A hash-comment file the
// table does not name falls back to bash; a named grammar missing its tables
// must not.
func grammarFor(filename string) (language *ts.Language, named bool) {
	load, ok := grammars[strings.ToLower(filepath.Ext(filename))]
	if !ok {
		return nil, false
	}
	return load(), true
}

// languageFor answers the grammar to read a file with. It is nil when the
// grammar has no parse tables, which it reports, and nil for a file of unknown
// syntax, which it does not. A `#` opens a directive in GLSL and a selector in
// CSS, so reading such a file as shell would cut code.
func languageFor(filename string) *ts.Language {
	language, named := grammarFor(filename)
	if named {
		if language == nil {
			reportMissingGrammar(filename)
		}
		return language
	}
	if !HashComments(filename) {
		return nil
	}
	if language := bash.Language(); language != nil {
		return language
	}
	reportMissingGrammar(filename)
	return nil
}

// reported holds the extensions already named, so an absent grammar prints a single time.
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

// A run breaks where the comments stop adjoining, where the left edge moves,
// and where a comment follows code.
func Runs(filename, src string) []Run {
	defer trace.Phase("treecomments/runs")()
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

func indentOf(src string, c Comment) int {
	start := strings.LastIndexByte(src[:c.Offset], '\n') + 1
	return len(src[start:c.Offset]) - len(strings.TrimLeft(src[start:c.Offset], " \t"))
}

// Extract returns every comment in the source, in source order.
//
// A syntax error yields comments anyway: tree-sitter recovers around it, so a
// rule still answers on a file mid-edit.
func Extract(filename, src string) []Comment {
	defer trace.Phase("treecomments/extract")()
	language := languageFor(filename)
	if language == nil {
		return nil
	}
	key := extractKey{language: language, sum: sha256.Sum256([]byte(src))}
	cached, ok := extracted.get(key)
	if !ok {
		cached = extract(language, src)
		extracted.put(key, cached)
	}
	out := slices.Clone(cached)
	if strings.HasSuffix(filename, "_test.go") {
		out = withoutExampleOutput(out)
	}
	if _, named := grammars[strings.ToLower(filepath.Ext(filename))]; !named {
		out = wholeLineOnly(src, out)
	}
	if IsCMake(filename) {
		out = cmakeLineComments(src, out)
	}
	return out
}

// wholeLineOnly keeps the comments that open their line. A file the bash
// grammar reads only as a fallback is not shell.
func wholeLineOnly(src string, comments []Comment) []Comment {
	kept := comments[:0]
	for _, c := range comments {
		if c.Col == indentOf(src, c) {
			kept = append(kept, c)
		}
	}
	return kept
}

// exampleOutput opens the comment that go test compares an example's output to.
var exampleOutput = regexp.MustCompile(`(?i)^//\s*(unordered )?output:`)

// IsExampleOutput reports a comment line that opens an example's output block in a Go test file.
func IsExampleOutput(line string) bool {
	return exampleOutput.MatchString(strings.TrimSpace(line))
}

// withoutExampleOutput drops each example's output block: the opening line and
// every comment line under it. go test reads it as data, so no rule may judge it.
func withoutExampleOutput(comments []Comment) []Comment {
	kept := comments[:0]
	inBlock, col, next := false, 0, 0
	for _, comment := range comments {
		switch {
		case exampleOutput.MatchString(comment.Text):
			inBlock, col = true, comment.Col
		case inBlock && comment.Line == next && comment.Col == col:
		default:
			inBlock = false
			kept = append(kept, comment)
		}
		next = comment.Line + comment.Lines
	}
	return kept
}

// extractKey names a parse by its grammar and the exact bytes it read.
type extractKey struct {
	language *ts.Language
	sum      [sha256.Size]byte
}

// extractCacheSize bounds the remembered parses. Every rule on a file asks for the same parse.
const extractCacheSize = 256

// extractCache remembers recent parses. The oldest entry goes first when it is full.
type extractCache struct {
	mu    sync.Mutex
	order []extractKey
	byKey map[extractKey][]Comment
}

var extracted = &extractCache{byKey: map[extractKey][]Comment{}}

func (c *extractCache) get(key extractKey) ([]Comment, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out, ok := c.byKey[key]
	return out, ok
}

func (c *extractCache) put(key extractKey, comments []Comment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.byKey[key]; ok {
		return
	}
	if len(c.order) == extractCacheSize {
		delete(c.byKey, c.order[0])
		c.order = c.order[1:]
	}
	c.order = append(c.order, key)
	c.byKey[key] = comments
}

// extract parses src and collects its comments.
func extract(language *ts.Language, src string) []Comment {
	parser := ts.NewParser()
	if !parser.SetLanguage(language) {
		return nil
	}
	root, parsed := parse(parser, src)
	if !parsed {
		return nil
	}
	defer trace.Phase("treecomments/walk")()
	var out []Comment
	collect(root, src, &out)
	return dropCgoPreamble(root, src, dropDockerDirectives(dropShebang(out)))
}

// A Dockerfile parser directive is a line the docker build reads, not prose.
var dockerDirective = regexp.MustCompile(`(?i)^#[ \t]*(syntax|escape|check)[ \t]*=[ \t]*\S+[ \t]*$`)

// dropDockerDirectives removes each Dockerfile parser directive from the comments.
func dropDockerDirectives(comments []Comment) []Comment {
	kept := comments[:0:0]
	for _, c := range comments {
		if c.Lines <= 1 && dockerDirective.MatchString(c.Text) {
			continue
		}
		kept = append(kept, c)
	}
	return kept
}

// parse runs the grammar over the source and answers the root to walk, or
// reports that there is none.
//
// It carries a phase name of its own. This is because a comment rule that
// reads slowly is usually paying for the parse under it rather than for the
// rule.
func parse(parser *ts.Parser, src string) (root ts.Node, parsed bool) {
	defer trace.Phase("treecomments/parse")()
	tree := parser.ParseString(nil, []byte(src))
	if tree == nil {
		return ts.Node{}, false
	}
	root = tree.RootNode()
	if root.IsNull() {
		return ts.Node{}, false
	}
	return root, true
}

// dropShebang removes the interpreter line a script opens on. Every grammar reads it as a comment. This is because it opens on the marker a single does. A repair that reflows the run beneath it welds the
// earliest sentence onto the interpreter.
func dropShebang(comments []Comment) []Comment {
	if len(comments) == 0 {
		return comments
	}
	head := comments[0]
	if head.Line != 1 || head.Col != 0 || head.Lines != 1 || !strings.HasPrefix(head.Text, "#!") {
		return comments
	}
	return comments[1:]
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
				// A Rust line comment node ends after its newline, and that newline is the next line's start.
				text := strings.TrimRight(src[start:end], "\r\n")
				if text == "" {
					text = src[start:end]
				}
				*out = append(*out, Comment{
					Text:   text,
					Offset: start,
					Line:   int(child.StartPoint().Row) + 1,
					Col:    int(child.StartPoint().Column),
					Lines:  strings.Count(text, "\n") + 1,
				})
			}
			continue
		}
		collect(child, src, out)
	}
}

// ready reports, per grammar, whether its generate step has run.
var ready = map[string]func() bool{
	"bash": bash.Ready, "clang": clang.Ready, "cpp": cpp.Ready,
	"golang": golang.Ready, "javascript": javascript.Ready,
	"rust": rust.Ready, "tsx": tsx.Ready, "typescript": typescript.Ready,
}

// Missing names the grammars built without their parse tables, in a stable
// order.
func Missing() []string {
	var out []string
	for _, name := range grammarNames {
		if !ready[name]() {
			out = append(out, name)
		}
	}
	return out
}

// grammarNames fixes the order Missing reports, so a message does not shuffle.
var grammarNames = []string{
	"bash", "clang", "cpp", "golang", "javascript", "rust", "tsx", "typescript",
}
