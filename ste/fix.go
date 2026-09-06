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
	// verbatimSpan matches the data strip also hides from Check.
	verbatimSpan = regexp.MustCompile("`[^`]*`|\\]\\([^)]*\\)")
	// semicolonRun matches a semicolon and the space that follows it.
	semicolonRun = regexp.MustCompile(`;[ \t]*`)
	// splicePattern shares checkSplices' conjunctions, so they cannot disagree.
	splicePattern = regexp.MustCompile(`(?i),\s+(` + strings.Join(spliceConjunctions, "|") + `)\s+`)
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
	return breakAt(prose, semicolonRun, func(string) string { return "" })
}

// fixSplices writes a period for the comma. The conjunction survives, and it
// opens the new sentence.
func fixSplices(prose string) string {
	return breakAt(prose, splicePattern, func(match string) string {
		return capitalize(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(match), ",")))
	})
}

// breakAt rewrites every joiner the pattern matches as a sentence break. opener
// returns the word that starts the new sentence, empty when the joiner carries
// none, and then the word already in the text takes the capital.
func breakAt(prose string, pattern *regexp.Regexp, opener func(string) string) string {
	var out strings.Builder
	last := 0
	for _, joiner := range pattern.FindAllStringIndex(prose, -1) {
		out.WriteString(prose[last:joiner[0]])
		last = joiner[1]

		word := opener(prose[joiner[0]:joiner[1]])
		if word != "" {
			out.WriteString(". " + word + " ")
			continue
		}
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
