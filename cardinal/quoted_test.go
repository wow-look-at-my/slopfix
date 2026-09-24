package cardinal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The shape the exemption exists for: a comment documenting a rewrite quotes
// the text on each side of it, and reporting the number inside the marks is
// what lets a repair leave the same words on each side of it.
func TestAQuotedNumberCountsNothingHere(t *testing.T) {
	documented := `the tally rule rewrites "two goroutines" to "goroutines"`
	assert.Empty(t, texts(documented, Comment))

	// The same words unquoted are the comment's own claim about the code.
	assert.Equal(t, []string{"two"}, texts("the tally rule rewrites two goroutines", Comment))
}

// An apostrophe spells a contraction far more often than it opens a quotation,
// so it bounds nothing and the count after it is still a count.
func TestAnApostropheOpensNoQuotation(t *testing.T) {
	assert.Equal(t, []string{"three"}, texts("it doesn't wait for three workers", Comment))
}

// A mark with no partner bounds nothing, so everything after it stays readable
// to the rule.
func TestAnUnclosedQuoteMarkExemptsNothing(t *testing.T) {
	assert.Equal(t, []string{"three"}, texts(`it holds "three workers`, Comment))
}

// A span covers its marks, so a caller splicing around a span keeps them.
func TestQuotedSpansCoverTheirMarks(t *testing.T) {
	assert.Equal(t, []Span{{Start: 0, End: 4}}, QuotedSpans(`"ab" c`))
	assert.Equal(t, []Span{{Start: 0, End: 3}, {Start: 4, End: 7}}, QuotedSpans("`a` `b`"))
	assert.Empty(t, QuotedSpans("no marks here"))
}
