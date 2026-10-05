package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What a cut comment reads as a single time it is closed is stated
// in rules/comment-tails.xml, where somebody adding a case edits no Go.
func TestEveryCommentTailTestHolds(t *testing.T) {
	require.NotEmpty(t, tailsTable.Tests)

	for _, c := range tailsTable.Tests {
		t.Run(c.In, func(t *testing.T) {
			assert.Equal(t, c.Out, CloseProse(c.In))
		})
	}
}

// The class is the vocabulary, so a word outside it ends a comment.
func TestAnOrdinaryWordEndsAComment(t *testing.T) {
	assert.False(t, dangling.Contains("comment"))
	assert.True(t, dangling.Contains("the"))
}
