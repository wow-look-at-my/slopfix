// Package blamelanguage finds deflecting, blame-shifting wording in a closing
// message.
//
// It catches the message that shifts responsibility for code in this org's own
// repositories onto another author or an earlier point in time. Every
// repository here was written by the same hand, so there is no other author to
// shift it to. A provenance opener in place of a repair is the shape.
//
// It is the sibling of laziness, which finds a turn ending with the work
// undone. This finds the excuse offered for it.
package blamelanguage

import (
	"regexp"
	"strings"
)

// ID names this rule, on a report and on the command line alike.
const ID = "blame/deflection"

// Hit is a banned phrase found in a message.
type Hit struct {
	// ID names the rule, the way a compiler names a warning.
	ID string `json:"id"`
	// Tell says in words which shape fired.
	Tell string `json:"tell"`
	// Phrase is the offending wording, as the writer spelled it.
	Phrase string `json:"phrase"`
	// Sentence quotes the line the phrase sits on, for context.
	Sentence string `json:"sentence"`
	// Line is where the phrase sits, counting from the top.
	Line int `json:"line"`
}

// phrases is the table. It is data: extending this package is adding a row.
// Every entry is grounded in this org's own written convention, and the rest
// are direct synonyms of an entry already here, kept narrow rather than
// speculative.
var phrases = []string{
	"pre-existing",
	"preexisting",
	"not my fault",
	"not my problem",
	"not mine to fix",
	"not my responsibility",
	"worth your attention",
	"flagging this for you",
	"flagging this here",
	"left as-is",
	"someone should",
	"out of scope",
	"not caused by my change",
	"not caused by my diff",
	"you may want to",
	"that predates this session",
	"predates this session",
	"this was existing code",
	"i only copied it",
	"git blame shows",
	"not related to my change",
	"unrelated to my diff",
}

// matcher pairs a phrase with its compiled, case-insensitive pattern, built at
// startup rather than per call.
type matcher struct {
	text string
	re   *regexp.Regexp
}

var matchers = compile(phrases)

func compile(list []string) []matcher {
	out := make([]matcher, len(list))
	for i, p := range list {
		out[i] = matcher{text: p, re: regexp.MustCompile(`(?i)` + regexp.QuoteMeta(p))}
	}
	return out
}

// Check reports every banned phrase the message carries, in table order, each
// at its earliest occurrence.
//
// Whitespace collapses beforehand, so a phrase a markdown line wrap split still
// matches. The offset table is what lets a match found in the collapsed text
// still name the line it came from.
func Check(message string) []Hit {
	text, lines := assertedText(message)
	norm, offsets := normalizeWhitespace(text)
	var hits []Hit
	for _, m := range matchers {
		at := m.re.FindStringIndex(norm)
		if at == nil {
			continue
		}
		start := offsets[at[0]]
		hits = append(hits, Hit{
			ID:       ID,
			Tell:     "deflecting the work onto another author or an earlier time",
			Phrase:   spelled(text, start, offsets, at),
			Sentence: lineAt(text, start),
			Line:     lineOf(lines, start),
		})
	}
	return hits
}

// spelled is the phrase as the writer wrote it, read back out of the original
// text so its case survives and a line wrap inside it collapses. A match
// reaching the end of the collapsed text has no offset after it.
func spelled(text string, start int, offsets []int, at []int) string {
	end := len(text)
	if at[1] < len(offsets) {
		end = offsets[at[1]]
	}
	if start >= end || end > len(text) {
		return ""
	}
	return strings.Join(strings.Fields(text[start:end]), " ")
}

// normalizeWhitespace collapses each run of ASCII whitespace to a space.
// offsets[i] is the byte offset in text that produced byte i of the result.
func normalizeWhitespace(text string) (norm string, offsets []int) {
	var b strings.Builder
	inSpace := false
	for i := range len(text) {
		c := text[i]
		if isASCIISpace(c) {
			if !inSpace {
				b.WriteByte(' ')
				offsets = append(offsets, i)
				inSpace = true
			}
			continue
		}
		inSpace = false
		b.WriteByte(c)
		offsets = append(offsets, i)
	}
	return b.String(), offsets
}

func isASCIISpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}

// assertedText blanks what the message quotes rather than asserts, keeping
// every byte offset so a report still names the right line.
//
// Fenced code, indented code and a blockquote are exempt. This policy can then
// be written down, and a phrase can be explained, without tripping the rule. A
// guard that cannot survive its own documentation is a guard somebody turns
// off. An inline backtick span is NOT exempt: a deflection in backticks is the
// exact thing this catches.
func assertedText(message string) (text string, lines []int) {
	var b strings.Builder
	fence := ""
	for n, line := range strings.Split(message, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if marker := fenceMarker(trimmed); marker != "" {
			if fence == "" {
				fence = marker
			} else if marker == fence {
				fence = ""
			}
			line = strings.Repeat(" ", len(line))
		} else if fence != "" ||
			strings.HasPrefix(trimmed, ">") ||
			(strings.HasPrefix(line, "    ") && trimmed != "") {
			line = strings.Repeat(" ", len(line))
		}
		if n > 0 {
			b.WriteByte('\n')
			lines = append(lines, n+1)
		}
		b.WriteString(line)
		for range len(line) {
			lines = append(lines, n+1)
		}
	}
	return b.String(), lines
}

// fenceMarker returns "`" or "~" when a line opens or closes a code fence.
func fenceMarker(trimmed string) string {
	for _, m := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, m) {
			return m[:1]
		}
	}
	return ""
}

// lineOf answers which source line a byte offset in the asserted text came from.
func lineOf(lines []int, at int) int {
	if at < len(lines) {
		return lines[at]
	}
	if len(lines) > 0 {
		return lines[len(lines)-1]
	}
	return 1
}

// lineAt returns the line holding byte offset i, collapsed and bounded so a
// report quotes a readable fragment rather than a paragraph.
func lineAt(text string, i int) string {
	start := strings.LastIndexByte(text[:i], '\n') + 1
	end := strings.IndexByte(text[i:], '\n')
	if end < 0 {
		end = len(text)
	} else {
		end += i
	}
	line := strings.Join(strings.Fields(text[start:end]), " ")
	if len(line) > 160 {
		line = line[:157] + "..."
	}
	return line
}
