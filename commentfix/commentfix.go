// Package commentfix finds a number stated in a comment.
//
// A number in a comment counts what exists today, and the edit that adds an
// item leaves it wrong. Nothing recompiles a comment, so describing what the
// code does and letting the reader count is the repair.
//
// The rule reads the comments and nothing else, which is what lets it answer
// before a compiler starts, on a tree that does not compile at all. The
// generated-file marker and the directive form are the Go-specific parts.
//
// This is the comment substrate of a rule the document substrate shares. Which
// numbers count lives in cardinal, beside the prose policy that demands a frame
// earliest. What is here is the comment: where it sits, which of its lines a
// reader was written for, and where a finding lands on the screen.
package commentfix

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/trace"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// ID names this rule, on a report and on the command line alike.
const ID = "comments/number"

// Supported reports whether this rule reads a file of that name.
func Supported(filename string) bool {
	return treecomments.Supported(filename) || readsHash(filename)
}

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
	// Offset is the byte the number starts at, inside the comment node that holds it.
	Offset int
}

// generatedLine is the line marking a file as generated, in every spelling of a comment the rule reads.
var generatedLine = regexp.MustCompile(`^\s*(?://+|#+|/\*|<!--)?\s*Code generated .* DO NOT EDIT\.\s*(?:\*/|-->)?$`)

// Check returns every number stated in a comment of a source file.
//
// A file in a language the extractor has no syntax for yields no hits. So does
// a file that does not compile: nothing here parses the language, which is why
// the rule answers on a tree mid-edit, before any compiler will look at it.
func Check(filename, src string) []Hit {
	defer trace.Phase("rule/comments-number")()
	if IsGenerated(filename, src) {
		return nil
	}
	var hits []Hit
	for _, run := range treecomments.Runs(filename, src) {
		var lines []commentLine
		for _, comment := range run {
			for _, line := range commentLines(comment.Text) {
				lines = append(lines, commentLine{text: line.text, offset: comment.Offset + line.offset})
			}
		}
		for _, stanza := range stanzasOf(lines) {
			hits = append(hits, stanzaHits(src, stanza)...)
		}
	}
	return hits
}

// stanzasOf breaks a run's lines on a line with nothing after its marker. That
// is where the repair breaks a paragraph, so a quotation closes no later than it.
func stanzasOf(lines []commentLine) [][]commentLine {
	var out [][]commentLine
	var current []commentLine
	for _, line := range lines {
		if _, prose, ok := split(line.text); ok && prose == "" {
			if len(current) > 0 {
				out = append(out, current)
			}
			current = nil
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		out = append(out, current)
	}
	return out
}

// stanzaHits finds the numbers each line states, leaving out a number inside a
// quotation that opens on one line and closes on a later one. The repair reads
// the paragraph whole, so that number is quoted to it as well.
func stanzaHits(src string, stanza []commentLine) []Hit {
	var joined strings.Builder
	starts := make([]int, len(stanza))
	for i, line := range stanza {
		starts[i] = joined.Len()
		joined.WriteString(line.text)
		joined.WriteByte('\n')
	}
	quoted := cardinal.QuotedSpans(joined.String())
	var hits []Hit
	for i, line := range stanza {
		for _, found := range cardinal.Find(line.text, cardinal.Comment) {
			if inSpans(quoted, starts[i]+found.Offset) {
				continue
			}
			at := line.offset + found.Offset
			pos, col := lineAndColumn(src, at)
			hits = append(hits, Hit{Number: found.Text, Line: pos, Col: col, Offset: at})
		}
	}
	return hits
}

// inSpans reports whether offset sits inside one of spans.
func inSpans(spans []cardinal.Span, offset int) bool {
	for _, span := range spans {
		if offset >= span.Start && offset < span.End {
			return true
		}
	}
	return false
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

// IsGenerated reports whether the file carries the generated-code marker in its header.
//
// The header is where the marker counts: the same words further down are prose somebody wrote. It is read off the tree, so what counts as a comment is the grammar's answer rather than a guess at a line's opening bytes, and the header ends at the earliest comment the file separates from the top with code.
func IsGenerated(filename, src string) bool {
	defer trace.Phase("rule/generated-marker")()
	if !Supported(filename) {
		return markedGenerated(src)
	}
	end := 0
	for _, comment := range treecomments.Extract(filename, src) {
		if strings.TrimSpace(src[end:comment.Offset]) != "" {
			return false
		}
		for _, line := range strings.Split(comment.Text, "\n") {
			if generatedLine.MatchString(strings.TrimRight(line, "\r")) {
				return true
			}
		}
		end = comment.Offset + len(comment.Text)
	}
	return false
}

// markedGenerated reports whether the first line holding text is the marker. A
// document has no comment grammar to read a header from, so the marker heads
// the file on its own line, as an HTML comment in markdown.
func markedGenerated(src string) bool {
	for line := range strings.Lines(src) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		return generatedLine.MatchString(strings.TrimRight(line, "\r\n"))
	}
	return false
}

// commentLine is a line of a comment's text and where it starts inside it.
type commentLine struct {
	text   string
	offset int
}

// commentLines splits a comment token into its lines. A block comment carries
// several. A directive line is skipped, because it addresses a tool rather than
// a reader. A code row is skipped, because its digits are code, not counts.
func commentLines(lit string) []commentLine {
	var out []commentLine
	at := 0
	for _, text := range strings.Split(lit, "\n") {
		if !isDirective(text) && !codeRow(text) {
			out = append(out, commentLine{text: text, offset: at})
		}
		at += len(text) + 1
	}
	return out
}

// isDirective reports whether the line is a compiler or tool directive, such as
// //go:build. The colon form carries no prose to go stale.
func isDirective(text string) bool {
	return treecomments.IsDirective(text)
}
