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
// are something else. The merge gate reads a document with no frame either, and
// pays for it with a list of units it will not count.
//
// Those differences are the whole reason they looked like separate rules. They
// are fields of Substrate now, and the values sit beside each other below.
package cardinal

import (
	"regexp"

	"github.com/wow-look-at-my/go-containers/set"
)

// Token is what a substrate reports, and where it starts in the text.
type Token struct {
	Offset int
	Text   string
}

// Shape is what a substrate looks for.
type Shape int

const (
	// Quantity is a cardinal governing a plural noun, as prose writes a tally.
	Quantity Shape = iota
	// Number is a number standing on its own, as a comment writes it.
	Number
)

// Substrate is a kind of text, and what a number has to do inside it to count.
type Substrate struct {
	// Shape says which finder reads the text, and so which exemptions apply.
	Shape Shape
	// Frame says a quantity must sit in a sentence claiming the things are HERE.
	Frame bool
	// Words is the vocabulary of numbers spelled in letters.
	Words set.Set[string]
	// Exempt judges a matched quantity, and Shape Quantity reads it.
	Exempt []Exemption
	// ExemptToken judges the text around a token, and Shape Number reads it. A
	ExemptToken []TokenExemption

	// quantity matches a cardinal governing a plural noun, spelled as this
	quantity *regexp.Regexp
}

// Prose is a document's own voice, as the inventory-count rule reads it. The
var Prose = Substrate{
	Shape:  Quantity,
	Frame:  true,
	Words:  proseWords,
	Exempt: []Exemption{ContinuesANumber, FunctionWordGap},
}

// Gate is the same document, as the merge gate's stale-count rule reads it. It
var Gate = Substrate{
	Shape:    Quantity,
	Frame:    false,
	Words:    gateWords,
	Exempt:   []Exemption{Unit, InExpression},
	quantity: gateQuantity,
}

// Comment is a comment in a source file. It asks for no frame either, so every
// exemption a number can earn has to be named here.
var Comment = Substrate{
	Shape:       Number,
	Frame:       false,
	Words:       commentWords,
	ExemptToken: []TokenExemption{HTTPStatus, SectionRef, Money},
}

// Find returns every stated count the text carries, under that substrate.
func Find(text string, s Substrate) []Token {
	switch {
	case s.Shape == Number:
		return walk(text, s)
	case s.Frame:
		return framed(text, s)
	}
	return quantities(text, s)
}
