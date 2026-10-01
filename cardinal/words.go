// words.go is the vocabulary of numbers spelled in letters. Every substrate
// reads it from here.
package cardinal

import (
	"regexp"

	"github.com/wow-look-at-my/go-containers/set"
)

// proseAlt is the prose vocabulary as a regular expression alternation, which
// is the form the frames need.
var proseAlt = words("prose")

// proseWords is the same vocabulary as a set, for a caller asking about a word.
var proseWords = set.Of(numbersTable.WordsOf("prose")...)

// gateAlt is what the merge gate's stale-count rule reads.
var gateAlt = words("gate")

// gateWords is that vocabulary as a set.
var gateWords = set.Of(numbersTable.WordsOf("gate")...)

// commentWords are the numbers spelled as whole words. A word inside a name, as in oneShot, is not one.
var commentWords = set.Of(numbersTable.WordsOf("comment")...)

// Leading matches the cardinal at the front of a quantity, with the space after
var Leading = regexp.MustCompile(`(?i)^(?:\d{1,4}|` + proseAlt + `)\s+`)
