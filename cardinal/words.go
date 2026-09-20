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
// is the form the frames need.
var proseAlt = strings.Join(numbersTable.WordsOf("prose"), "|")

// proseWords is the same vocabulary as a set, for a caller asking about a word.
var proseWords = set.Of(numbersTable.WordsOf("prose")...)

// gateAlt is what the merge gate's stale-count rule reads, as an alternation.
var gateAlt = strings.Join(numbersTable.WordsOf("gate"), "|")

// gateWords is that vocabulary as a set.
var gateWords = set.Of(numbersTable.WordsOf("gate")...)

// commentWords are the numbers a comment spells.
var commentWords = set.Of(numbersTable.WordsOf("comment")...)

// Leading matches the cardinal at the front of a quantity, with the space after
var Leading = regexp.MustCompile(`(?i)^(?:\d{1,4}|` + proseAlt + `)\s+`)
