// Package cardinal decides whether a number in a piece of text is a stated
// count: a number that is true today and wrong after the next commit.
//
// A single rule, over several substrates. A document's prose and a source
// comment go stale the same way, and each used to carry a private copy of the
// walk that says so. What differs is not the rule. It is how much a substrate
// has to say before a number counts as a claim about what is here.
//
// Prose REQUIRES A FRAME. A document legitimately carries numbers that count
// nothing -- a version, a port, an example -- so the sentence has to claim the
// things belong here before the number is a tally. A comment REQUIRES NONE: a
// number written beside code is nearly always a count of what the code holds,
// so the cardinal alone is the finding, and the exemptions carry the cases that
// are something else.
//
// That difference is the whole reason they looked like separate rules. It is a
// field of Substrate now, and the values sit beside each other below.
package cardinal

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// Token is what a substrate reports, and where it starts in the text.
//
// The text differs by substrate on purpose. Prose reports the whole quantity,
// the cardinal and the noun it governs, because the repair cuts the cardinal
// off the front of it. A comment reports the number alone: there is nothing
// there to cut.
type Token struct {
	Offset int
	Text   string
}

// Substrate is a kind of text, and how much framing a number needs inside it.
type Substrate struct {
	// Frame says a number must sit in a sentence claiming the things are HERE.
	Frame bool
	// Words is the vocabulary of numbers spelled in letters.
	Words set.Set[string]
	// Exempt are the shapes around a token that carry a number without counting
	// anything. A token's own shape -- a URL, a qualified name -- is judged
	// inside the number test instead, where its order against the digit test
	// decides the answer.
	Exempt []Exemption
}

// Prose is a document's own voice. It requires a frame, and it applies no
// token exemption: the frame is what keeps an ordinary number out.
var Prose = Substrate{Frame: true, Words: proseWords}

// Comment is a comment in a source file. It requires no frame, so every
// exemption a number can earn has to be named here instead.
var Comment = Substrate{
	Frame:  false,
	Words:  commentWords,
	Exempt: []Exemption{HTTPStatus, SectionRef, Money},
}

// Find returns every stated count the text carries, under that substrate.
func Find(text string, s Substrate) []Token {
	if s.Frame {
		return framed(text)
	}
	return walk(text, s)
}

// isInventory rejects a quantity reached through a function word. A bare
// adjective run happily swallows "of the format".
func isInventory(phrase string) bool {
	words := strings.Fields(strings.ToLower(phrase))
	if len(words) < 2 {
		return false
	}
	for _, w := range words[1:len(words)-1] {
		if gapStopWords.Contains(w) {
			return false
		}
	}
	return true
}

// gapStopWords are function words proving the noun after them is not what the
// cardinal counts.
var gapStopWords = set.Of[string](
	"of", "the", "a", "an", "in", "on", "to", "for", "and", "or", "is", "are",
	"was", "were", "that", "this", "with", "from", "by", "at", "as", "but",
	"if", "so", "than", "then", "when", "while", "not", "no", "it", "its",
)

// continuesANumber reports a match that is the tail of a longer number, so a
// version string is not read as a count.
func continuesANumber(text string, start int) bool {
	if start == 0 {
		return false
	}
	c := text[start-1]
	return c == '.' || (c >= '0' && c <= '9')
}
