// Package commentnumbers finds a number stated in a comment.
//
// A number in a comment is a count of what exists today, and the edit that adds
// an item leaves it wrong. Nothing recompiles a comment, so the stale sentence
// survives every build. Describing what the code does, and letting the reader
// count, is the repair.
//
// The rule reads the comments and nothing else. The source package finds them
// by walking the bytes rather than by parsing the language, which is what lets
// the check answer before a compiler starts, on a tree that does not compile at
// all.
//
// The generated-file marker and the directive form are the Go-specific parts.
// Everything else applies to every language the adapter knows.
//
// This package is the comment substrate of a rule the document substrate
// shares. Which numbers count lives in cardinal, beside the prose policy, which
// demands a frame before it reads a number as a tally. What is here is the
// comment: where it sits, which of its lines a reader was written for, and
// where a finding lands on the screen.
package commentnumbers

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/source"
)

// ID names this rule, on a report and on the command line alike.
const ID = "comments/number"

// Supported reports whether this rule reads a file of that name.
func Supported(filename string) bool { return source.Supported(filename) }

// Remedy is what every finding asks the author to do instead. A reference to a
// numbered section is the case rewriting the sentence does not cover, so it
// also names the slug that survives a renumbering edit.
const Remedy = "a number in a comment is a count of what exists today, " +
	"and the edit that adds an item leaves it wrong: describe what the code does and let the reader count. " +
	"To point at a section of a spec or a document, cite its unique slug or its heading text, never its position: " +
	"the slug survives the edit that inserts a section above it, and a section sign (§) marks a citation that has no slug"

// Hit is a number found in a comment, at the character a reader sees.
type Hit struct {
	Number string
	Line   int
	Col    int
}

// generatedMarker opens the line marking a file as generated.
const generatedMarker = "// Code generated "

// generatedSuffix closes that same line.
const generatedSuffix = " DO NOT EDIT."

// Check returns every number stated in a comment of a source file.
//
// A file in a language the extractor has no syntax for yields no hits. So does
// a file that does not compile: nothing here parses the language, which is why
// the rule answers on a tree mid-edit, before any compiler will look at it.
func Check(filename, src string) []Hit {
	if IsGenerated(src) {
		return nil
	}
	var hits []Hit
	for _, comment := range source.Extract(filename, src) {
		for _, line := range commentLines(comment.Text) {
			for _, found := range cardinal.Find(line.text, cardinal.Comment) {
				at := comment.Offset + line.offset + found.Offset
				pos, col := lineAndColumn(src, at)
				hits = append(hits, Hit{Number: found.Text, Line: pos, Col: col})
			}
		}
	}
	return hits
}

// lineAndColumn answers where a byte offset sits, counting from the top and the
// left of the file.
func lineAndColumn(src string, at int) (line, col int) {
	if at > len(src) {
		at = len(src)
	}
	line = 1 + strings.Count(src[:at], "\n")
	start := strings.LastIndexByte(src[:at], '\n') + 1
	return line, at - start + 1
}

// IsGenerated reports whether the file carries the generated-code marker.
func IsGenerated(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, generatedMarker) && strings.HasSuffix(line, generatedSuffix) {
			return true
		}
		if strings.HasPrefix(line, "package ") {
			return false
		}
	}
	return false
}

// commentLine is a line of a comment's text and where it starts inside it.
type commentLine struct {
	text   string
	offset int
}

// commentLines splits a comment token into its lines. A block comment carries
// several, and a directive line is skipped: it addresses a tool rather than a
// reader.
func commentLines(lit string) []commentLine {
	var out []commentLine
	at := 0
	for _, text := range strings.Split(lit, "\n") {
		if !isDirective(text) {
			out = append(out, commentLine{text: text, offset: at})
		}
		at += len(text) + 1
	}
	return out
}

// isDirective reports whether the line is a compiler or tool directive, such as
// //go:build. The colon form carries no prose to go stale.
func isDirective(text string) bool {
	text = strings.TrimSpace(text)
	rest, found := strings.CutPrefix(text, "//")
	if !found || rest == "" || strings.HasPrefix(rest, " ") {
		return false
	}
	name, _, found := strings.Cut(rest, ":")
	if !found || name == "" {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// The token walk that reads these lines, and every exemption a number can earn,
// live in cardinal. Only the comment's own shape is here.

