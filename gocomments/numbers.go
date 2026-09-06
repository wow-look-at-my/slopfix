// Package gocomments finds a number stated in a Go comment.
//
// A number in a comment is a count of what exists today, and the edit that adds
// an item leaves it wrong. Nothing recompiles a comment, so the stale sentence
// survives every build. Describing what the code does, and letting the reader
// count, is the repair.
//
// The rule reads the comments and nothing else. It runs the Go SCANNER, which
// tokenizes a file without building an AST, resolving an import or loading a
// package. That is what lets the check answer before a compiler starts, on a
// tree that does not compile at all.
package gocomments

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
)

// ID names this rule, on a report and on the command line alike.
const ID = "gocomments/comment-number"

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

// numberWords are the numbers spelled as words: the cardinals, the ordinals
// that index a list, and the words for a repeat count. A word joined to other
// letters is a name (oneShot, someone), so only a whole word counts.
var numberWords = set.Of(
	"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
	"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen",
	"seventeen", "eighteen", "nineteen", "twenty", "thirty", "forty", "fifty",
	"sixty", "seventy", "eighty", "ninety", "hundred", "thousand", "million",
	"billion", "trillion", "dozen",
	"once", "twice", "thrice",
	"first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth",
	"ninth", "tenth", "eleventh", "twelfth", "thirteenth", "fourteenth",
	"fifteenth", "sixteenth", "seventeenth", "eighteenth", "nineteenth",
	"twentieth", "thirtieth", "fortieth", "fiftieth", "sixtieth", "seventieth",
	"eightieth", "ninetieth", "hundredth", "thousandth",
)

// generatedMarker opens the line that marks a file as generated. Its author is
// a program, and telling a program to write differently is not a repair.
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
	for _, comment := range Extract(filename, src) {
		for _, line := range commentLines(comment.Text) {
			for _, found := range numbersIn(line.text) {
				at := comment.Offset + line.offset + found.offset
				pos, col := lineAndColumn(src, at)
				hits = append(hits, Hit{Number: found.text, Line: pos, Col: col})
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

// commentToken is a run of name characters and where it starts in the line.
type commentToken struct {
	offset int
	text   string
}

// nameMarkers join an identifier, an import path or a label into a name.
const nameMarkers = "._/:"

// numbersIn finds every number in the text of a comment line.
func numbersIn(text string) []commentToken {
	var found []commentToken
	toks := tokensIn(text)
	for i, tok := range toks {
		if isHTTPStatus(toks, i) || isSectionRef(text, tok) || isMoney(text, tok) {
			continue
		}
		if hit, ok := tokenNumber(tok); ok {
			found = append(found, hit)
		}
	}
	return found
}

// httpStatusPrefix is the word that names the digits after it as a status code.
const httpStatusPrefix = "HTTP"

// statusCodeDigits is the width of an HTTP status code.
const statusCodeDigits = len("500")

// isHTTPStatus reports whether the token at i is an HTTP status code. The
// prefix is what separates the digits of a protocol answer from a count wearing
// the same shape.
func isHTTPStatus(toks []commentToken, i int) bool {
	if i == 0 {
		return false
	}
	prefix := strings.Trim(toks[i-1].text, nameMarkers)
	return strings.EqualFold(prefix, httpStatusPrefix) && isStatusCode(toks[i].text)
}

// isStatusCode reports whether text is the bare digits of a status code. A name
// marker at either end is the punctuation of the sentence, since a token keeps
// the period that ends it.
func isStatusCode(text string) bool {
	text = strings.Trim(text, nameMarkers)
	if len(text) != statusCodeDigits {
		return false
	}
	for _, r := range text {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// sectionSign marks the number after it as a citation of a section.
const sectionSign = '§'

// isSectionRef reports whether the token sits behind a section sign, which
// cites a section of a document instead of counting anything here.
func isSectionRef(text string, tok commentToken) bool {
	before := strings.TrimRight(text[:tok.offset], " \t")
	last, size := utf8.DecodeLastRuneInString(before)
	return size > 0 && last == sectionSign
}

// currencySign marks the digits against it as an amount rather than a count.
const currencySign = '$'

// isMoney reports whether the token opens with digits carrying a currency sign
// directly against them. An amount is a value, and only the amount is exempt.
func isMoney(text string, tok commentToken) bool {
	last, size := utf8.DecodeLastRuneInString(text[:tok.offset])
	if size == 0 || last != currencySign {
		return false
	}
	return unicode.IsDigit(rune(tok.text[0]))
}

// tokensIn splits a comment line into runs of name characters, so a URL, an
// import path and a version each stay whole.
func tokensIn(text string) []commentToken {
	var toks []commentToken
	start := -1
	for i, r := range text {
		if isNameRune(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			toks = append(toks, commentToken{start, text[start:i]})
			start = -1
		}
	}
	if start >= 0 {
		toks = append(toks, commentToken{start, text[start:]})
	}
	return toks
}

// isNameRune reports whether r can sit inside a technical name.
func isNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(nameMarkers+"-", r)
}

// tokenNumber reports the number a token carries, if it carries any.
func tokenNumber(tok commentToken) (commentToken, bool) {
	if strings.Contains(tok.text, "://") {
		return commentToken{}, false // the digits of a URL are part of it
	}
	if hit, ok := digitNumber(tok); ok {
		return hit, true
	}
	if isQualifiedName(tok.text) {
		return commentToken{}, false // sync.Once and net/http name themselves
	}
	return wordNumber(tok)
}

// isQualifiedName reports whether a marker sits BETWEEN name characters, which
// is what separates an identifier from a sentence that ends on a word.
func isQualifiedName(text string) bool {
	runes := []rune(text)
	for i := 1; i < len(runes)-1; i++ {
		if !strings.ContainsRune(nameMarkers, runes[i]) {
			continue
		}
		if isWordRune(runes[i-1]) && isWordRune(runes[i+1]) {
			return true
		}
	}
	return false
}

// isWordRune reports whether r is a letter or a digit.
func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// digitNumber reports the digits a token spells as a number. A digit touching a
// letter is part of a name (sha256, amd64, 10ms); an ordinal suffix makes it a
// number again.
func digitNumber(tok commentToken) (commentToken, bool) {
	runes, offsets := runesOf(tok.text)
	for i := 0; i < len(runes); i++ {
		if !unicode.IsDigit(runes[i]) {
			continue
		}
		end := i
		for end < len(runes) && unicode.IsDigit(runes[end]) {
			end++
		}
		if !touchesLetter(runes, i, end) || hasOrdinalSuffix(runes, end) {
			return commentToken{tok.offset + offsets[i], string(runes[i:end])}, true
		}
		i = end
	}
	return commentToken{}, false
}

// wordNumber reports the number a token spells in letters.
func wordNumber(tok commentToken) (commentToken, bool) {
	runes, offsets := runesOf(tok.text)
	for i := 0; i < len(runes); i++ {
		if !unicode.IsLetter(runes[i]) {
			continue
		}
		end := i
		for end < len(runes) && unicode.IsLetter(runes[end]) {
			end++
		}
		word := string(runes[i:end])
		if numberWords.Contains(strings.ToLower(word)) {
			return commentToken{tok.offset + offsets[i], word}, true
		}
		i = end
	}
	return commentToken{}, false
}

// runesOf splits s into its runes alongside each rune's byte offset, so a
// finding can point at the character a reader sees.
func runesOf(s string) ([]rune, []int) {
	runes := make([]rune, 0, len(s))
	offsets := make([]int, 0, len(s))
	for i, r := range s {
		runes = append(runes, r)
		offsets = append(offsets, i)
	}
	return runes, offsets
}

// touchesLetter reports whether a letter sits directly against the run, or
// against a hyphen that does. A hyphen binds a compound name as tightly as
// nothing at all: a stated width and a stated duration are the same kind of
// fact, and neither counts what sits below it.
func touchesLetter(runes []rune, start, end int) bool {
	return letterAt(runes, start-1, -1) || letterAt(runes, end, 1)
}

// letterAt reports a letter at i, or past a hyphen sitting there.
func letterAt(runes []rune, i, step int) bool {
	if i < 0 || i >= len(runes) {
		return false
	}
	if unicode.IsLetter(runes[i]) {
		return true
	}
	if runes[i] != '-' {
		return false
	}
	next := i + step
	return next >= 0 && next < len(runes) && unicode.IsLetter(runes[next])
}

// ordinalSuffixes are what a digit wears when it is still a number.
var ordinalSuffixes = set.Of("st", "nd", "rd", "th")

// hasOrdinalSuffix reports whether an ordinal suffix, and nothing else, follows
// the digits at end.
func hasOrdinalSuffix(runes []rune, end int) bool {
	suffix := end + len("st")
	if suffix > len(runes) {
		return false
	}
	if suffix < len(runes) && unicode.IsLetter(runes[suffix]) {
		return false
	}
	return ordinalSuffixes.Contains(strings.ToLower(string(runes[end:suffix])))
}
