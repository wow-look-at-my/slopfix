package treecomments

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The docker build reads a parser directive, so a rule must never cut it.
func TestADockerDirectiveIsNotAComment(t *testing.T) {
	src := testADockerDirectiveIsNotACommentMd

	got := Extract("Dockerfile", src)
	require.Len(t, got, 1)
	assert.Equal(t, "# what this image holds", got[0].Text)
}

// A compose file carries the directive inside a dockerfile_inline block.
func TestADockerDirectiveInComposeIsNotAComment(t *testing.T) {
	src := testADockerDirectiveInComposeIsNotACommentYml

	assert.Empty(t, Extract("compose.yaml", src))
}

// Prose that only mentions a directive stays a comment.
func TestProseAboutADirectiveStaysAComment(t *testing.T) {
	src := "# the syntax = line picks the frontend\nFROM alpine\n"

	assert.Len(t, Extract("Dockerfile", src), 1)
}

//go:embed testdata/test-a-docker-directive-is-not-a-comment.md
var testADockerDirectiveIsNotACommentMd string

//go:embed testdata/test-a-docker-directive-in-compose-is-not-a-comment.yml
var testADockerDirectiveInComposeIsNotACommentYml string
