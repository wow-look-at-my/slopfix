package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rules/comment-tails.xml states what a cut comment reads as once it is
// closed. Somebody who adds a case there edits no Go.
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
