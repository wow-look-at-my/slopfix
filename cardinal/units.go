// units.go exempts a measurement from the count rules.
//
// A cardinal before a unit states a magnitude, and no set sits behind it to go
// stale. The words are the unit class in rules/english-classes.xml. A word
// added there needs no Go.
package cardinal

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
)

// numbersTable carries the classes the number rules read.
var numbersTable = table.MustLoad(rules.FS, "numbers")

// IsUnit reports whether a word names a unit of measure.
func IsUnit(word string) bool {
	return numbersTable.Lexicon().Is(strings.Trim(strings.ToLower(word), ".,;:()"), "unit")
}

// GovernsAUnit exempts a quantity whose noun is a unit of measure.
func GovernsAUnit(_ string, q Match) bool {
	return IsUnit(q.Noun)
}

// TokenGovernsAUnit is GovernsAUnit for a substrate that walks tokens: it asks
// about the word after the number rather than about a matched noun.
func TokenGovernsAUnit(text string, toks []Token, i int) bool {
	tok := toks[i]
	rest := text[min(tok.Offset+len(tok.Text), len(text)):]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return false
	}
	return IsUnit(fields[0])
}
