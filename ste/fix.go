package ste

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Fix applies the repair Check names. A long sentence is left alone,
// because splitting it needs a writer who knows the point.
func Fix(text string) string {
	return fixProse(text, func(prose string) string {
		return fixSplices(fixSemicolons(fixWords(prose)))
	})
}

var (
	// verbatimSpan matches the data strip also hides from Check. An entity ends
	// in a semicolon, which a repair must not touch.
	verbatimSpan = regexp.MustCompile(
		codeSpan.String() + `|` + linkTarget.String() + `|` + entity.String())
	// semicolonRun matches a semicolon and the space that follows it.
	semicolonRun = regexp.MustCompile(`;[ \t]*`)
	// spliceComma matches the comma checkSplices reports, and nothing past it.
	spliceComma = regexp.MustCompile(`,\s+`)
)

// fixProse applies repair to the prose of text, leaving each verbatim span
// alone. A semicolon inside `a; b` is the thing the sentence documents.
func fixProse(text string, repair func(string) string) string {
	var out strings.Builder
	last := 0
	for _, span := range verbatimSpan.FindAllStringIndex(text, -1) {
		out.WriteString(repair(text[last:span[0]]))
		out.WriteString(text[span[0]:span[1]])
		last = span[1]
	}
	out.WriteString(repair(text[last:]))
	return out.String()
}

// fixWords writes the approved word for each banned word, and keeps the
// capitalization the source used.
func fixWords(prose string) string {
	return wordPattern.ReplaceAllStringFunc(prose, func(word string) string {
		lower := strings.ToLower(word)
		replacement, banned := contractions[lower]
		if !banned {
			replacement, banned = modals[lower]
		}
		if !banned {
			return word
		}
		if isCapitalized(word) {
			return capitalize(replacement)
		}
		return replacement
	})
}

// fixSemicolons writes the period the semicolon stands in for.
func fixSemicolons(prose string) string {
	return breakAt(prose, semicolonRun.FindAllStringIndex(prose, -1))
}

// fixSplices writes the period each spliced comma stands in for.
//
// The comma is found the way checkSplices finds it, guard included, so the
// repair covers exactly what the check reports. Only the comma is rewritten:
// a conjunction after it survives and opens the new sentence.
func fixSplices(prose string) string {
	var commas [][]int
	for _, loc := range commaSplice.FindAllStringSubmatchIndex(prose, -1) {
		if bare := loc[2] < 0; bare && !isClause(clauseBefore(prose, loc[0])) {
			continue
		}
		end := loc[0] + len(spliceComma.FindString(prose[loc[0]:]))
		commas = append(commas, []int{loc[0], end})
	}
	return breakAt(prose, commas)
}

// breakAt rewrites each joiner span as a sentence break, and gives the word
// after it the capital a sentence opens with.
func breakAt(prose string, joiners [][]int) string {
	var out strings.Builder
	last := 0
	for _, joiner := range joiners {
		if joiner[0] < last {
			continue
		}
		out.WriteString(prose[last:joiner[0]])
		last = joiner[1]
		if last == len(prose) {
			// The joiner ends the text, so no word follows to open a sentence.
			out.WriteString(".")
			continue
		}
		next, width := utf8.DecodeRuneInString(prose[last:])
		out.WriteString(". " + string(unicode.ToUpper(next)))
		last += width
	}
	out.WriteString(prose[last:])
	return out.String()
}

func isCapitalized(word string) bool {
	first, _ := utf8.DecodeRuneInString(word)
	return unicode.IsUpper(first)
}

func capitalize(s string) string {
	first, width := utf8.DecodeRuneInString(s)
	if width == 0 {
		return s
	}
	return string(unicode.ToUpper(first)) + s[width:]
}
