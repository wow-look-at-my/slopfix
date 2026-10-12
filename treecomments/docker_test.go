package treecomments

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The docker build reads a parser directive, so a rule must never cut it.
func TestADockerDirectiveIsNotAComment(t *testing.T) {
	src := "# syntax=docker/dockerfile:1\n#check=skip=JSONArgsRecommended\n# ESCAPE = `\n\n# what this image holds\nFROM alpine\n"

	got := Extract("Dockerfile", src)
	require.Len(t, got, 1)
	assert.Equal(t, "# what this image holds", got[0].Text)
}

// A compose file carries the directive inside a dockerfile_inline block.
func TestADockerDirectiveInComposeIsNotAComment(t *testing.T) {
	src := "services:\n  app:\n    build:\n      dockerfile_inline: |\n        # syntax=docker/dockerfile:1\n        FROM alpine\n"

	assert.Empty(t, Extract("compose.yaml", src))
}

// Prose that only mentions a directive stays a comment.
func TestProseAboutADirectiveStaysAComment(t *testing.T) {
	src := "# the syntax = line picks the frontend\nFROM alpine\n"

	assert.Len(t, Extract("Dockerfile", src), 1)
}
