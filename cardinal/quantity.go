// quantity.go is the finder both document substrates read: a cardinal
// governing a plural noun. It also holds what each of them exempts.
//
// Both spell the shape with different tolerances, and the spellings sit here
// together rather than in the packages that read them. Neither is derived from
// the other. They are what both rules have always matched, and a fold that
// merged them would move verdicts on the merge gate.
package cardinal

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
)

// proseQuantity is the inventory-count spelling: a plural cardinal, up
const proseQuantity = `(?:\d{1,4}|\b(?:` + proseAlt + `))` +
	`\s+(?:[a-z][a-z-]*\s+){0,3}?[a-z][a-z-]{2,}s\b`

// gateQuantity is the merge gate's spelling. It reads a shorter list of words,
// any run of digits, and a shorter adjective gap.
var gateQuantity = regexp.MustCompile(`(?i)\b(?:` + gateAlt + `|[0-9]+)` +
	`\s+(?:[a-z-]+\s+){0,2}?[a-z]+s\b`)

// Match is a quantity the pattern found, and the parts an exemption asks about.
type Match struct {
	// At is where the cardinal starts in the text.
	At int
	// Text is the whole quantity, the cardinal and the noun it governs.
	Text string
	// Noun is the plural noun the cardinal governs.
	Noun string
}

// Exemption reports whether a matched quantity counts nothing here.
type Exemption func(text string, q Match) bool

// quantities returns every quantity in the text, for a substrate that asks for
// no frame around it.
func quantities(text string, s Substrate) []Token {
	var out []Token
	for _, at := range s.quantity.FindAllStringIndex(text, -1) {
		q := quantityAt(text, at[0], at[1])
		if exemptQuantity(text, q, s.Exempt) {
			continue
		}
		out = append(out, Token{Offset: q.At, Text: q.Text})
	}
	return out
}

// quantityAt reads the parts of the span the pattern matched. The noun is the
// last word of it, because every spelling of the shape ends on the noun.
func quantityAt(text string, start, end int) Match {
	phrase := text[start:end]
	noun := ""
	if fields := strings.Fields(phrase); len(fields) > 0 {
		noun = strings.ToLower(fields[len(fields)-1])
	}
	return Match{At: start, Text: phrase, Noun: noun}
}

func exemptQuantity(text string, q Match, rules []Exemption) bool {
	for _, rule := range rules {
		if rule(text, q) {
			return true
		}
	}
	return false
}

// ContinuesANumber exempts a match that is the tail of a longer number, so a
// version string is not read as a count.
func ContinuesANumber(text string, q Match) bool {
	if q.At == 0 {
		return false
	}
	c := text[q.At-1]
	return c == '.' || (c >= '0' && c <= '9')
}

// FunctionWordGap exempts a quantity reached through a function word. A bare
// adjective run happily swallows "of the format".
func FunctionWordGap(_ string, q Match) bool {
	words := strings.Fields(strings.ToLower(q.Text))
	if len(words) < 2 {
		return true
	}
	for _, w := range words[1 : len(words)-1] {
		if gapStopWords.Contains(w) {
			return true
		}
	}
	return false
}

// gapStopWords are function words proving the noun after them is not what the
// cardinal counts.
var gapStopWords = set.Of[string](
	"of", "the", "a", "an", "in", "on", "to", "for", "and", "or", "is", "are",
	"was", "were", "that", "this", "with", "from", "by", "at", "as", "but",
	"if", "so", "than", "then", "when", "while", "not", "no", "it", "its",
)

// Unit exempts a noun that is measured rather than counted. A budget in
func Unit(_ string, q Match) bool { return units.Contains(q.Noun) }

// units are the nouns that name a measure.
var units = set.Of[string](
	"bits", "bytes", "kilobytes", "megabytes", "gigabytes", "characters", "chars", "runes",
	"words", "lines", "columns", "rows", "spaces", "digits", "seconds", "minutes", "hours",
	"days", "weeks", "months", "years", "milliseconds", "microseconds", "nanoseconds",
	"pixels", "points", "percent", "times", "levels", "degrees",
)

// InExpression exempts a number that is arithmetic rather than a count. The
// digits in an expression or a range name no set of items.
func InExpression(text string, q Match) bool {
	if q.At == 0 {
		return false
	}
	before := []rune(text[:q.At])
	prev := before[len(before)-1]
	switch prev {
	case '-', '−', '+', '/', '*', '=', '.', ',', '_':
		return true
	}
	return unicode.IsDigit(prev)
}
