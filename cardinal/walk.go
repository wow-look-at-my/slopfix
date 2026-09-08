// walk.go is the comment half: the token walk over a line, and the shapes that
// carry a number without counting anything here.
package cardinal

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
)

// Exemption reports whether the token at i is a number this substrate reads as
// something other than a count.
type Exemption func(text string, toks []Token, i int) bool

// nameMarkers join an identifier, an import path or a label into a name.
const nameMarkers = "._/:"

// walk finds every number in a line the substrate reads without a frame.
func walk(text string, s Substrate) []Token {
	var found []Token
	toks := tokensIn(text)
	for i, tok := range toks {
		if exempt(text, toks, i, s.Exempt) {
			continue
		}
		if hit, ok := tokenNumber(tok, s.Words); ok {
			found = append(found, hit)
		}
	}
	return found
}

func exempt(text string, toks []Token, i int, rules []Exemption) bool {
	for _, rule := range rules {
		if rule(text, toks, i) {
			return true
		}
	}
	return false
}

// httpStatusPrefix is the word that names the digits after it as a status code.
const httpStatusPrefix = "HTTP"

// statusCodeDigits is the width of an HTTP status code.
const statusCodeDigits = len("500")

// HTTPStatus exempts a status code. The prefix is what separates the digits of
// a protocol answer from a count wearing the same shape.
func HTTPStatus(_ string, toks []Token, i int) bool {
	if i == 0 {
		return false
	}
	prefix := strings.Trim(toks[i-1].Text, nameMarkers)
	return strings.EqualFold(prefix, httpStatusPrefix) && isStatusCode(toks[i].Text)
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

// SectionRef exempts a token behind a section sign, which cites a section of a
// document instead of counting anything here.
func SectionRef(text string, toks []Token, i int) bool {
	before := strings.TrimRight(text[:toks[i].Offset], " \t")
	last, size := utf8.DecodeLastRuneInString(before)
	return size > 0 && last == sectionSign
}

// currencySign marks the digits against it as an amount rather than a count.
const currencySign = '$'

// Money exempts a token opening with digits that carry a currency sign directly
// against them. An amount is a value, and only the amount is exempt.
func Money(text string, toks []Token, i int) bool {
	tok := toks[i]
	last, size := utf8.DecodeLastRuneInString(text[:tok.Offset])
	if size == 0 || last != currencySign {
		return false
	}
	return unicode.IsDigit(rune(tok.Text[0]))
}

// tokensIn splits a line into runs of name characters, so a URL, an import path
// and a version each stay whole.
func tokensIn(text string) []Token {
	var toks []Token
	start := -1
	for i, r := range text {
		if isNameRune(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			toks = append(toks, Token{start, text[start:i]})
			start = -1
		}
	}
	if start >= 0 {
		toks = append(toks, Token{start, text[start:]})
	}
	return toks
}

// isNameRune reports whether r can sit inside a technical name.
func isNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(nameMarkers+"-", r)
}

// tokenNumber reports the number a token carries, if it carries any.
//
// The order is load-bearing. A URL is never a number. Digits are tested next,
// so a version reports the part of it no name binds. Only then does a qualified
// name exempt what is left, which is a token spelling a number in letters.
func tokenNumber(tok Token, words set.Set[string]) (Token, bool) {
	if strings.Contains(tok.Text, "://") {
		return Token{}, false // the digits of a URL are part of it
	}
	if hit, ok := digitNumber(tok); ok {
		return hit, true
	}
	if isQualifiedName(tok.Text) {
		return Token{}, false // sync.Once and net/http name themselves
	}
	return wordNumber(tok, words)
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
func digitNumber(tok Token) (Token, bool) {
	runes, offsets := runesOf(tok.Text)
	for i := 0; i < len(runes); i++ {
		if !unicode.IsDigit(runes[i]) {
			continue
		}
		end := i
		for end < len(runes) && unicode.IsDigit(runes[end]) {
			end++
		}
		if !touchesLetter(runes, i, end) || hasOrdinalSuffix(runes, end) {
			return Token{tok.Offset + offsets[i], string(runes[i:end])}, true
		}
		i = end
	}
	return Token{}, false
}

// wordNumber reports the number a token spells in letters.
func wordNumber(tok Token, words set.Set[string]) (Token, bool) {
	runes, offsets := runesOf(tok.Text)
	for i := 0; i < len(runes); i++ {
		if !unicode.IsLetter(runes[i]) {
			continue
		}
		end := i
		for end < len(runes) && unicode.IsLetter(runes[end]) {
			end++
		}
		word := string(runes[i:end])
		if words.Contains(strings.ToLower(word)) {
			return Token{tok.Offset + offsets[i], word}, true
		}
		i = end
	}
	return Token{}, false
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

// touchesLetter reports a letter directly against the run, or past a binding
// hyphen: a compound name states a fact rather than a count.
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
