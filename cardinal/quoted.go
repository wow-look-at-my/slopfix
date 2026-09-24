// quoted.go exempts a number sitting inside a quotation.
//
// A quotation carries text the sentence is talking ABOUT rather than text the
// sentence asserts. The clearest case is a comment documenting a rule by
// quoting what the rule reads and what it writes: repairing inside the marks
// edits the example, and an example of a rewrite in which nothing differs says
// nothing at all.
//
// The apostrophe is not a quote mark here. It spells a contraction and a
// possessive far more often than it opens a quotation, so reading it that way
// would exempt the rest of any line carrying the word "doesn't".
package cardinal

import "strings"

// quoteMarks open a quotation and close it with the same character.
const quoteMarks = "\"`"

// Span is a half-open byte range of a piece of text.
type Span struct {
	Start, End int
}

// QuotedSpans reports every quotation in text, quote marks and all. An opening
// mark with no closer bounds nothing, so it is passed over.
func QuotedSpans(text string) []Span {
	var out []Span
	for i := 0; i < len(text); i++ {
		if strings.IndexByte(quoteMarks, text[i]) < 0 {
			continue
		}
		rest := strings.IndexByte(text[i+1:], text[i])
		if rest < 0 {
			continue
		}
		end := i + 1 + rest + 1
		out = append(out, Span{Start: i, End: end})
		i = end - 1
	}
	return out
}

// InQuotes reports whether the byte at offset sits inside a quotation.
func InQuotes(text string, offset int) bool {
	for _, span := range QuotedSpans(text) {
		if offset >= span.Start && offset < span.End {
			return true
		}
	}
	return false
}

// Quoted exempts a token inside a quotation, whatever number it spells.
func Quoted(text string, toks []Token, i int) bool {
	return InQuotes(text, toks[i].Offset)
}
