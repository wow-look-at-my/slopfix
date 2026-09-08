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
const proseAlt = `two|three|four|five|six|seven|eight|nine|ten|` +
	`eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|` +
	`nineteen|twenty|thirty|forty|fifty|sixty|seventy|eighty|ninety|dozen`

// proseWords is the same vocabulary as a set, for a caller asking about a word.
var proseWords = set.Of(strings.Split(proseAlt, "|")...)

// gateAlt is what the merge gate's stale-count rule reads. It stops short of
const gateAlt = `two|three|four|five|six|seven|eight|nine|ten|eleven|twelve`

// gateWords is that vocabulary as a set.
var gateWords = set.Of(strings.Split(gateAlt, "|")...)

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
var Leading = regexp.MustCompile(`(?i)^(?:\d{1,4}|` + proseAlt + `)\s+`)
