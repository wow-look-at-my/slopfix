// words.go is the vocabulary of numbers spelled in letters. Every substrate
// reads it from here, and it reads the words themselves from rules/, so adding
// one stays an edit to XML that needs no Go.
package cardinal

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

//go:generate go run github.com/wow-look-at-my/slopfix/cmd/rulegen -rules ../rules -for numbers -package cardinal -out numbers.gen.go

// proseAlt is the prose vocabulary as a regular expression alternation, which
// is the form the frames need. rulegen joins the words, so a pattern built out
// of this stays a constant.
const proseAlt = numbersAltProse

// proseWords is the same vocabulary as a set, for a caller asking about a word.
var proseWords = set.Of(strings.Split(proseAlt, "|")...)

// gateAlt is what the merge gate's stale-count rule reads, as an alternation.
const gateAlt = numbersAltGate

// gateWords is that vocabulary as a set.
var gateWords = set.Of(strings.Split(gateAlt, "|")...)

// commentWords are the numbers a comment spells.
var commentWords = set.Of(strings.Split(numbersAltComment, "|")...)

// Leading matches the cardinal at the front of a quantity, with the space after
var Leading = regexp.MustCompile(`(?i)^(?:\d{1,4}|` + proseAlt + `)\s+`)
