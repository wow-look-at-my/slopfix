// units.go answers whether a word names a unit of measure.
//
// A measurement is still a stated count and still reported. What separates it
// is the repair. The tally rule rewrites "goroutines" to "goroutines", and
// doing that to "30 seconds" leaves a sentence with no magnitude in it. A word
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
	return InClass(word, "unit")
}

// InClass reports whether a word belongs to a class the numbers table names.
func InClass(word, class string) bool {
	return numbersTable.Lexicon().Is(strings.Trim(strings.ToLower(word), ".,;:()"), class)
}
