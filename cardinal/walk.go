// walk.go is the comment half: the token walk over a line, and the shapes that
// carry a number without counting anything here.
package cardinal

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
)

// TokenExemption reports whether the token at i is a number this substrate
type TokenExemption func(text string, toks []Token, i int) bool

// nameMarkers join an identifier, an import path or a label into a name.
const nameMarkers = "._/:"

// walk finds every number in a line the substrate reads without a frame.
func walk(text string, s Substrate) []Token {
	var found []Token
	toks := tokensIn(text)
	for i, tok := range toks {
		if exemptToken(text, toks, i, s.ExemptToken) {
			continue
		}
		if hit, ok := tokenNumber(tok, s.Words); ok {
			found = append(found, hit)
		}
	}
	return found
}

func exemptToken(text string, toks []Token, i int, rules []TokenExemption) bool {
	for _, rule := range rules {
		if rule(text, toks, i) {
			return true
		}
	}
	return false
}

// LabeledToken exempts digits after a label word, as in "HTTP 403" or
// "migration 014". The word names one item, and the digits identify it.
func LabeledToken(_ string, toks []Token, i int) bool {
	if i == 0 || !allDigits(strings.Trim(toks[i].Text, nameMarkers)) {
		return false
	}
	return InClass(strings.Trim(toks[i-1].Text, nameMarkers), "label")
}

// MeasureToken exempts a number whose plural noun takes a singular verb: "43
// cols is less than the cap". The phrase is one measured amount, a value of a
// test or a format, and a cut leaves "cols is".
func MeasureToken(_ string, toks []Token, i int) bool {
	if i+2 >= len(toks) || !strings.HasSuffix(strings.ToLower(toks[i+1].Text), "s") {
		return false
	}
	return singularVerbs.Contains(strings.ToLower(toks[i+2].Text))
}

// WordSizeToken exempts a word size, "32 bits" and its kin. The width is fixed
// by the machine, so no edit to a set makes it stale.
func WordSizeToken(_ string, toks []Token, i int) bool {
	if i+1 >= len(toks) {
		return false
	}
	switch toks[i].Text {
	case "8", "16", "32", "64", "128":
		noun := strings.ToLower(toks[i+1].Text)
		return noun == "bits" || noun == "bit"
	}
	return false
}

// singularVerbs agree with one amount, never with a plural tally.
var singularVerbs = set.Of("is", "was", "has", "does", "fits", "equals")

// exitStatusPrefixes name the digits after them as a status.
var exitStatusPrefixes = set.Of("exit", "exits", "exited", "status", "errno", "signal")

// ExitStatus exempts the digits of an exit status. The rule reports a count of
// what exists today, because the edit that adds an item leaves the count wrong.
// A status is a VALUE the program answers with, and no edit moves it.
func ExitStatus(_ string, toks []Token, i int) bool {
	if i == 0 || !allDigits(strings.Trim(toks[i].Text, nameMarkers)) {
		return false
	}
	return exitStatusPrefixes.Contains(strings.ToLower(strings.Trim(toks[i-1].Text, nameMarkers)))
}

// allDigits reports whether text is a bare run of digits.
func allDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// literalMarkers stand against the digits of a literal value.
const literalMarkers = `="'`

// comparisonMarkers close an operator that takes a value: `=`, `<`, `>`, and every operator built from them.
const comparisonMarkers = `=<>`

// Literal exempts the digits of a value the code is written against. That is
// an env marker an assignment sets, or the quoted string a parser reads as
// unset. It is also the operand of a comparison such as `used > 0`. An added item leaves a count
// wrong, and leaves a value alone.
func Literal(text string, toks []Token, i int) bool {
	tok := toks[i]
	if !allDigits(strings.Trim(tok.Text, `"'`)) {
		return false
	}
	last, size := utf8.DecodeLastRuneInString(text[:tok.Offset])
	if size > 0 && strings.ContainsRune(literalMarkers, last) {
		return true
	}
	last, size = utf8.DecodeLastRuneInString(strings.TrimRight(text[:tok.Offset], " \t"))
	return size > 0 && strings.ContainsRune(comparisonMarkers, last)
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

// listLead is what may stand before a list marker: indentation and comment markers.
const listLead = " \t/#*;"

// ListMarker exempts the digits that open a numbered list item, such as "1." or
// "2)". The marker orders the items and counts nothing.
func ListMarker(text string, toks []Token, i int) bool {
	tok := toks[i]
	if strings.Trim(text[:tok.Offset], listLead) != "" {
		return false
	}
	digits := strings.TrimSuffix(tok.Text, ".")
	if !allDigits(digits) {
		return false
	}
	rest := text[tok.Offset+len(digits):]
	return strings.HasPrefix(rest, ". ") || strings.HasPrefix(rest, ") ")
}

// Operand exempts a number written as code: the argument of a call, or an operand
// of a star or a plus. Its value is a size the format fixes.
func Operand(text string, toks []Token, i int) bool {
	tok := toks[i]
	if !allDigits(tok.Text) {
		return false
	}
	before, _ := utf8.DecodeLastRuneInString(text[:tok.Offset])
	after, _ := utf8.DecodeRuneInString(text[tok.Offset+len(tok.Text):])
	if before == '*' || before == '+' || after == '*' {
		return true
	}
	if before != '(' || after != ')' {
		return false
	}
	call, size := utf8.DecodeLastRuneInString(text[:tok.Offset-1])
	return size > 0 && isNameRune(call)
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
func tokenNumber(tok Token, words set.Set[string]) (Token, bool) {
	if strings.Contains(tok.Text, "://") {
		return Token{}, false // the digits of a URL are part of it
	}
	if isQualifiedName(tok.Text) && strings.IndexFunc(tok.Text, unicode.IsLetter) >= 0 {
		return Token{}, false
	}
	if hit, ok := digitNumber(tok); ok {
		return hit, true
	}
	if isQualifiedName(tok.Text) {
		return Token{}, false // sync.a single time and net/http name
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
		// A hyphen binds a number word into a compound that names a shape: "two-line".
		if words.Contains(strings.ToLower(word)) && !letterAt(runes, i-1, -1) && !letterAt(runes, end, 1) {
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
