// Package source is a substrate adapter. It answers where the prose is in a
// source file, by extracting the comments and nothing else.
//
// The syntax table spans the C family and the hash family, so a rule written
// against this adapter reaches every language in it. Nothing here is specific to
// any language, and a rule that reads a comment reads it the same way whatever
// wrote the file.
package source

import (
	"path/filepath"
	"strings"
)

// syntax is how a family of languages spells a comment and a string. A comment
// marker inside a literal is data, so the delimiters matter as much.
type syntax struct {
	line      []string
	blockOpen string
	blockEnd  string
	strings   []string
	// raw is a string delimiter that honours no escape, such as Go's backtick.
	raw []string
}

// cFamily is the // and /* */ pair, which most of this org's languages use.
var cFamily = syntax{
	line:      []string{"//"},
	blockOpen: "/*",
	blockEnd:  "*/",
	strings:   []string{`"`, `'`},
}

// goSyntax adds the raw literal, where a backslash escapes nothing.
var goSyntax = syntax{
	line:      []string{"//"},
	blockOpen: "/*",
	blockEnd:  "*/",
	strings:   []string{`"`, `'`},
	raw:       []string{"`"},
}

// hashFamily is the `#` line comment, with no block form.
var hashFamily = syntax{
	line:    []string{"#"},
	strings: []string{`"`, `'`},
}

// byExtension maps a file to the syntax it is written in. A language absent
// here is skipped rather than guessed at: a wrong guess reports a string
// literal as prose, and a rule nobody trusts is a rule nobody keeps.
var byExtension = map[string]syntax{
	".go":    goSyntax,
	".c":     cFamily,
	".h":     cFamily,
	".cc":    cFamily,
	".cpp":   cFamily,
	".hpp":   cFamily,
	".rs":    cFamily,
	".java":  cFamily,
	".js":    cFamily,
	".mjs":   cFamily,
	".cjs":   cFamily,
	".ts":    cFamily,
	".tsx":   cFamily,
	".jsx":   cFamily,
	".swift": cFamily,
	".kt":    cFamily,
	".scala": cFamily,
	".zig":   cFamily,
	".py":    hashFamily,
	".rb":    hashFamily,
	".sh":    hashFamily,
	".bash":  hashFamily,
	".zsh":   hashFamily,
	".yml":   hashFamily,
	".yaml":  hashFamily,
	".toml":  hashFamily,
	".tf":    hashFamily,
	".just":  hashFamily,
	".mk":    hashFamily,
}

// Comment is a comment's text and where it begins in the source.
type Comment struct {
	Text   string
	Offset int
}

// Supported reports whether this rule reads a file of that name.
func Supported(filename string) bool {
	_, ok := syntaxFor(filename)
	return ok
}

// syntaxFor answers the comment syntax a file is written in.
func syntaxFor(filename string) (syntax, bool) {
	base := filepath.Base(filename)
	if base == "Makefile" || base == "Dockerfile" || base == "Justfile" {
		return hashFamily, true
	}
	s, ok := byExtension[strings.ToLower(filepath.Ext(filename))]
	return s, ok
}

// Extract returns every comment in the source, in order.
//
// This walks the bytes rather than parsing the language. A parser answers a
// question this rule never asks, and every parser worth using for it wants cgo,
// which the org's fat-APE builds cannot take. What the walk must get right is
// narrow: never read a comment marker that sits inside a string, and never read
// a string delimiter that sits inside a comment.
func Extract(filename, src string) []Comment {
	s, ok := syntaxFor(filename)
	if !ok {
		return nil
	}
	var out []Comment
	for i := 0; i < len(src); {
		rest := src[i:]

		if marker, found := openerAt(rest, s.line); found {
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				end = len(rest)
			}
			out = append(out, Comment{Text: rest[:end], Offset: i})
			i += end
			_ = marker
			continue
		}

		if s.blockOpen != "" && strings.HasPrefix(rest, s.blockOpen) {
			end := strings.Index(rest[len(s.blockOpen):], s.blockEnd)
			if end < 0 {
				out = append(out, Comment{Text: rest, Offset: i})
				break
			}
			stop := len(s.blockOpen) + end + len(s.blockEnd)
			out = append(out, Comment{Text: rest[:stop], Offset: i})
			i += stop
			continue
		}

		if delim, found := openerAt(rest, s.raw); found {
			i += skipLiteral(rest, delim, false)
			continue
		}
		if delim, found := openerAt(rest, s.strings); found {
			i += skipLiteral(rest, delim, true)
			continue
		}
		i++
	}
	return out
}

// openerAt reports the delimiter the text opens with.
func openerAt(text string, delims []string) (string, bool) {
	for _, d := range delims {
		if d != "" && strings.HasPrefix(text, d) {
			return d, true
		}
	}
	return "", false
}

// skipLiteral returns how many bytes the literal opening this text occupies. An
// unterminated literal consumes the rest, which stops a stray quote from
// turning the remaining file into prose.
func skipLiteral(text, delim string, escapes bool) int {
	at := len(delim)
	for at < len(text) {
		if escapes && text[at] == '\\' {
			at += 2
			continue
		}
		if strings.HasPrefix(text[at:], delim) {
			return at + len(delim)
		}
		at++
	}
	return len(text)
}
