// frame.go is what the inventory-count rule asks for beyond a quantity: a
// sentence claiming the things being counted are HERE. Lacking such a shape, a
// number in a document counts nothing. The merge gate's substrate asks for no
// frame, so it reads a bare quantity anywhere in the line.
package cardinal

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
)

// quantity is the shape a frame governs. quantity.go spells it.
const quantity = proseQuantity

// words alternates a class of english-frames.xml, for a pattern built at load.
func words(class string) string {
	return strings.Join(numbersTable.WordsOf(class), "|")
}

// possessiveFrame is a determiner claiming the things belong here, as in "this
// repo's plugins" or "the payload's steps".
var possessiveFrame = regexp.MustCompile(
	`(?i)\b(?:this|these|our|the)\s+(?:[a-z][a-z-]*\s+){0,2}?[a-z][a-z-]*'s\s+(` + quantity + `)`)

// havingFrame is a verb asserting possession or extent, as in "it ships hooks"
// or "there are sections". A measurement inside a frame is a count like any
// other: a budget gets raised and a suite gets slower, and it reads with more
// authority than a tally because an instrument looks to have produced it.
var havingFrame = regexp.MustCompile(
	`(?i)\b(?:` + words("framing") +
		`|(?:` + words("durative") + `)\s+(?:` + words("duration") + `)` +
		`|(?:` + words("existential") + `)\s+(?:` + words("existence") + `)` +
		`)\s+(?:(?:` + words("hedge") + `)\s+)?(` + quantity + `)`)

// deicticFrame points inside the document, as in "the rules below". The count
// is of what this page shows, so editing the page breaks it.
var deicticFrame = regexp.MustCompile(
	`(?i)\b(?:the|these|those)\s+(` + quantity + `)\s+(?:\S+\s+){0,2}?(?:` + words("deictic") + `)\b`)

var frames = []*regexp.Regexp{possessiveFrame, havingFrame, deicticFrame}

// framed returns every quantity a frame governs, without repeating a phrase.
//
// The frames are read in turn, so the same phrase can match several of them. A
// phrase is reported only where it matches earliest.
func framed(text string, s Substrate) []Token {
	var out []Token
	seen := set.New[string]()
	for _, frame := range frames {
		for _, at := range frame.FindAllStringSubmatchIndex(text, -1) {
			q := quantityAt(text, at[2], at[3])
			if exemptQuantity(text, q, s.Exempt) || seen.Contains(q.Text) {
				continue
			}
			seen.Add(q.Text)
			out = append(out, Token{Offset: q.At, Text: q.Text})
		}
	}
	return out
}
