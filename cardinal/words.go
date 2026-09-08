// words.go is the vocabulary of numbers spelled in letters. Every substrate
// reads it from here.
package cardinal

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// proseAlt is the prose vocabulary as a regular expression alternation, which
// is the form the frames need.
//
// The singular is deliberately absent: in English prose it is overwhelmingly a
// pronoun, and matching it reports far more good writing than bad. The ordinals
// and the large scales are absent for the same reason. A comment reads the
// wider table below, where a bare cardinal is already the finding.
const proseAlt = `two|three|four|five|six|seven|eight|nine|ten|` +
	`eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|` +
	`nineteen|twenty|thirty|forty|fifty|sixty|seventy|eighty|ninety|dozen`

// proseWords is the same vocabulary as a set, for a caller asking about a word.
var proseWords = set.Of(strings.Split(proseAlt, "|")...)

// commentWords are the numbers spelled as words: the cardinals, the ordinals
// that index a list, and the words for a repeat count. A word joined to other
// letters is a name (oneShot, someone), so only a whole word counts.
var commentWords = set.Of(
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

// Leading matches the cardinal at the front of a quantity, with the space after
// it. Cutting exactly that is the prose repair.
var Leading = regexp.MustCompile(`(?i)^(?:\d{1,4}|` + proseAlt + `)\s+`)
