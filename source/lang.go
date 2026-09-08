// lang.go is the syntax table. Every language this adapter reads is a row in
// it, and a shared engine reads every row. A rule therefore behaves the same way
// whatever wrote the file, which is the whole point of the adapter.
package source

import (
	"path/filepath"
	"strings"
)

// blockSpec is a block comment's delimiters. Rust, Swift and Zig nest theirs,
// so an inner open has to be counted rather than ignored.
type blockSpec struct {
	open   string
	close  string
	nested bool
}

// stringSpec is a literal form. escape says a backslash escapes the next
// byte. multiline says a newline is part of the literal rather than the end of
// it. charLike marks a quote that also spells a lifetime or a plain word, so it
// only opens a literal when a close is near.
type stringSpec struct {
	open      string
	close     string
	escape    bool
	multiline bool
	charLike  bool
}

// syntax is a language's spelling of a comment, a literal and a block.
type syntax struct {
	line  []string
	block []blockSpec
	str   []stringSpec
	// wordStartComment marks a language where a marker only opens a comment at
	// the start of a word. `echo a#b` passes a literal hash to echo.
	wordStartComment bool
	// heredoc marks the shell form, where `<<WORD` runs to a line holding WORD.
	heredoc bool
	// rustRaw marks the `r#"..."#` form, whose close carries the same hashes.
	rustRaw bool
	// indent marks a language whose blocks are bounded by indentation.
	indent bool
	// open and close are the bracket pairs that bound a block.
	open  string
	close string
}

// brackets are the pairs every language in this table nests.
const (
	openBrackets  = "{(["
	closeBrackets = "})]"
)

var (
	cStrings = []stringSpec{
		{open: `"`, close: `"`, escape: true},
		{open: `'`, close: `'`, escape: true, charLike: true},
	}

	cFamily = syntax{
		line:  []string{"//"},
		block: []blockSpec{{open: "/*", close: "*/"}},
		str:   cStrings,
		open:  openBrackets,
		close: closeBrackets,
	}

	// goSyntax adds the backtick literal, where a backslash escapes nothing.
	goSyntax = syntax{
		line:  []string{"//"},
		block: []blockSpec{{open: "/*", close: "*/"}},
		str: append([]stringSpec{
			{open: "`", close: "`", multiline: true},
		}, cStrings...),
		open:  openBrackets,
		close: closeBrackets,
	}

	// rustSyntax nests its block comments and carries the raw literal.
	rustSyntax = syntax{
		line:    []string{"//"},
		block:   []blockSpec{{open: "/*", close: "*/", nested: true}},
		str:     cStrings,
		rustRaw: true,
		open:    openBrackets,
		close:   closeBrackets,
	}

	// nestedC is the C family for the languages that nest a block comment.
	nestedC = syntax{
		line:  []string{"//"},
		block: []blockSpec{{open: "/*", close: "*/", nested: true}},
		str:   cStrings,
		open:  openBrackets,
		close: closeBrackets,
	}

	// shellSyntax carries the heredoc and the raw single-quoted literal.
	shellSyntax = syntax{
		line: []string{"#"},
		str: []stringSpec{
			{open: `"`, close: `"`, escape: true, multiline: true},
			{open: `'`, close: `'`, multiline: true},
		},
		wordStartComment: true,
		heredoc:          true,
		open:             "{(",
		close:            "})",
	}

	// pythonSyntax has the triple-quoted literal and indentation blocks.
	pythonSyntax = syntax{
		line: []string{"#"},
		str: []stringSpec{
			{open: `"""`, close: `"""`, escape: true, multiline: true},
			{open: `'''`, close: `'''`, escape: true, multiline: true},
			{open: `"`, close: `"`, escape: true},
			{open: `'`, close: `'`, escape: true},
		},
		wordStartComment: true,
		indent:           true,
		open:             openBrackets,
		close:            closeBrackets,
	}

	// hashFamily is the plain `#` line comment with no block form.
	hashFamily = syntax{
		line: []string{"#"},
		str: []stringSpec{
			{open: `"`, close: `"`, escape: true},
			{open: `'`, close: `'`, escape: true},
		},
		wordStartComment: true,
		open:             openBrackets,
		close:            closeBrackets,
	}

	// indentHash is the `#` family whose blocks are bounded by indentation.
	indentHash = syntax{
		line: []string{"#"},
		str: []stringSpec{
			{open: `"`, close: `"`, escape: true},
			{open: `'`, close: `'`, multiline: true},
		},
		wordStartComment: true,
		indent:           true,
		open:             openBrackets,
		close:            closeBrackets,
	}

	// luaSyntax spells both its comment forms with a pair of hyphens.
	luaSyntax = syntax{
		line:  []string{"--"},
		block: []blockSpec{{open: "--[[", close: "]]"}},
		str: []stringSpec{
			{open: `"`, close: `"`, escape: true},
			{open: `'`, close: `'`, escape: true},
			{open: "[[", close: "]]", multiline: true},
		},
		open:  "{(",
		close: "})",
	}

	// sqlSyntax pairs the hyphen line comment with the C block form.
	sqlSyntax = syntax{
		line:  []string{"--"},
		block: []blockSpec{{open: "/*", close: "*/"}},
		str: []stringSpec{
			{open: `'`, close: `'`, escape: true, multiline: true},
			{open: `"`, close: `"`, escape: true},
		},
		open:  "(",
		close: ")",
	}
)

// byExtension maps a file to the syntax it is written in. A language absent
// here is skipped rather than guessed at: a wrong guess reports a string
// literal as prose, and a rule nobody trusts is a rule nobody keeps.
var byExtension = map[string]syntax{
	".go":     goSyntax,
	".c":      cFamily,
	".h":      cFamily,
	".cc":     cFamily,
	".cpp":    cFamily,
	".cxx":    cFamily,
	".hpp":    cFamily,
	".hh":     cFamily,
	".m":      cFamily,
	".mm":     cFamily,
	".cs":     cFamily,
	".java":   cFamily,
	".js":     cFamily,
	".mjs":    cFamily,
	".cjs":    cFamily,
	".ts":     cFamily,
	".tsx":    cFamily,
	".jsx":    cFamily,
	".php":    cFamily,
	".dart":   cFamily,
	".rs":     rustSyntax,
	".swift":  nestedC,
	".zig":    cFamily,
	".kt":     nestedC,
	".kts":    nestedC,
	".scala":  nestedC,
	".groovy": cFamily,
	".py":     pythonSyntax,
	".pyi":    pythonSyntax,
	".rb":     hashFamily,
	".sh":     shellSyntax,
	".bash":   shellSyntax,
	".zsh":    shellSyntax,
	".ksh":    shellSyntax,
	".yml":    indentHash,
	".yaml":   indentHash,
	".toml":   hashFamily,
	".tf":     hashFamily,
	".hcl":    hashFamily,
	".just":   hashFamily,
	".mk":     hashFamily,
	".pl":     hashFamily,
	".pm":     hashFamily,
	".r":      hashFamily,
	".lua":    luaSyntax,
	".sql":    sqlSyntax,
	".el":     {line: []string{";"}, str: cStrings, open: "(", close: ")"},
	".lisp":   {line: []string{";"}, str: cStrings, open: "(", close: ")"},
	".clj":    {line: []string{";"}, str: cStrings, open: openBrackets, close: closeBrackets},
	".ex":     hashFamily,
	".exs":    hashFamily,
	".erl":    {line: []string{"%"}, str: cStrings, open: "{([", close: "})]"},
	".tex":    {line: []string{"%"}, str: cStrings, open: "{", close: "}"},
	".vim":    {line: []string{`"`}, str: []stringSpec{{open: `'`, close: `'`}}, open: "(", close: ")"},
}

// byBaseName covers the files a build reads by name rather than by extension.
var byBaseName = map[string]syntax{
	"Makefile":       hashFamily,
	"makefile":       hashFamily,
	"GNUmakefile":    hashFamily,
	"Dockerfile":     hashFamily,
	"Containerfile":  hashFamily,
	"Justfile":       hashFamily,
	"justfile":       hashFamily,
	"Vagrantfile":    hashFamily,
	"Gemfile":        hashFamily,
	"Rakefile":       hashFamily,
	"CMakeLists.txt": hashFamily,
	"PKGBUILD":       shellSyntax,
	".bashrc":        shellSyntax,
	".zshrc":         shellSyntax,
	".profile":       shellSyntax,
}

// Supported reports whether this adapter reads a file of that name.
func Supported(filename string) bool {
	_, ok := syntaxFor(filename)
	return ok
}

// syntaxFor answers the syntax a file is written in.
func syntaxFor(filename string) (syntax, bool) {
	base := filepath.Base(filename)
	if s, ok := byBaseName[base]; ok {
		return s, true
	}
	if strings.HasPrefix(base, "Dockerfile.") || strings.HasPrefix(base, "Makefile.") {
		return hashFamily, true
	}
	s, ok := byExtension[strings.ToLower(filepath.Ext(filename))]
	return s, ok
}
