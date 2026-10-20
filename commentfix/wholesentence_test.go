package commentfix

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// zeroDiffIndicators is a comment that heads a run of declarations with no
// blank line between them. Weighed against the first const alone, its repair
// cut it to "... and the description opens." and dropped both sentences
// after it.
//
//go:embed testdata/zero-diff-indicators.js
var zeroDiffIndicators string

// A comment heads the code that runs on below it until a blank line, so it is
// weighed against all of that code.
func TestACommentIsWeighedAgainstTheCodeParagraphItHeads(t *testing.T) {
	assert.Empty(t, CheckLength("handlers.ts", zeroDiffIndicators))
	out, _ := FixLength("handlers.ts", zeroDiffIndicators)
	assert.Equal(t, zeroDiffIndicators, out)
}

// A blank line ends the paragraph a comment heads.
func TestABlankLineEndsTheParagraphAComments(t *testing.T) {
	src := strings.Replace(zeroDiffIndicators, "export const ZERO_DIFF_BODY_NOTE", "\nexport const ZERO_DIFF_BODY_NOTE", 1)
	require.NotEmpty(t, CheckLength("handlers.ts", src))
}

// The cut removes whole trailing sentences from the end of the block. What is
// left is cut back to a clause boundary, so the text stays whole clauses.
func TestTheCutKeepsWholeSentences(t *testing.T) {
	src := strings.Replace(zeroDiffIndicators, "export const ZERO_DIFF_BODY_NOTE", "\nexport const ZERO_DIFF_BODY_NOTE", 1)
	out, changed := FixLength("handlers.ts", src)
	require.True(t, changed)
	assert.Empty(t, CheckLength("handlers.ts", out))
	assert.NotContains(t, commentProse(out), "A failed PATCH", "the last whole sentence goes:\n%s", out)
	assert.Contains(t, commentProse(out), "the title gains.", "what stays is a whole clause:\n%s", out)
}

// A single sentence that spans more lines than the code under it is a finding.
// The repair cuts it back to a clause the sentence holds.
func TestASingleSentenceOverItsCodeIsCut(t *testing.T) {
	src := "package p\n\n" +
		"// cacheKeyFor returns the stable composite lookup key built from the resolved descriptor\n" +
		"// set index plus binding slot plus array element offset plus sampler identity hash value.\n" +
		"func cacheKeyFor() int { return 0 }\n"
	require.NotEmpty(t, CheckLength("p.go", src))
	out, changed := FixLength("p.go", src)
	require.True(t, changed)
	assert.Equal(t, "package p\n\n// cacheKeyFor returns the stable composite lookup key.\nfunc cacheKeyFor() int { return 0 }\n", out)
}
