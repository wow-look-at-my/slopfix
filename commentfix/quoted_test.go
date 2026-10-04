package commentfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A comment that documents a rewrite quotes the text on each side of it. The
// repair reads a quotation as the sentence's subject rather than its claim, so
// the example still differs on each side of it after a pass.
func TestAQuotedExampleSurvivesTheRepair(t *testing.T) {
	src := "// The tally rule rewrites \"two goroutines\" to \"goroutines\".\nfunc f() {}\n"
	repair := fix(t, src)
	assert.False(t, repair.Changed, "the repair reached inside a quotation: %q", repair.Text)
	assert.Equal(t, src, repair.Text)
}

// The same sentence unquoted is the comment's own count, and the repair takes
// it. Without this the case above would pass on prose no rule ever touched.
func TestTheSameSentenceUnquotedIsStillRepaired(t *testing.T) {
	bare := fix(t, "// The tally rule rewrites two goroutines.\nfunc f() {}\n")
	assert.True(t, bare.Changed)
	assert.NotContains(t, bare.Text, "two goroutines")
}
