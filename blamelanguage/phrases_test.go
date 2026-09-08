package blamelanguage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARunOfWhitespaceCollapsesToOneSpace(t *testing.T) {
	assert.NotEmpty(t, Check("out   of\n\n  scope for this change."))
}

func TestOrdinaryProseIsNotAHit(t *testing.T) {
	cases := []string{
		"I found the bug in auth.go and fixed it; the suite is green.",
		"The scope of this change is the parser, not the lexer.",
		"Someone on the team asked for this feature last quarter.",
		"This file predates the rewrite of the storage layer.",
	}
	for _, text := range cases {
		assert.Empty(t, Check(text), "expected no hit for %q", text)
	}
}

func TestAReportQuotesTheLineAndNumbersIt(t *testing.T) {
	hits := Check("first line\nthat bug is pre-existing and not mine.\nlast line")
	require.Len(t, hits, 1)
	assert.Equal(t, "that bug is pre-existing and not mine.", hits[0].Sentence)
	assert.Equal(t, 2, hits[0].Line)
	assert.NotEmpty(t, hits[0].Tell)
}

func TestFenceMarker(t *testing.T) {
	assert.Equal(t, "`", fenceMarker("```go"))
	assert.Equal(t, "~", fenceMarker("~~~"))
	assert.Equal(t, "", fenceMarker("plain text"))
}

func TestUnclosedFenceSwallowsTheRest(t *testing.T) {
	assert.Empty(t, Check("intro\n```\nout of scope\n"))
}

func TestNormalizeWhitespaceCollapsesRuns(t *testing.T) {
	norm, offsets := normalizeWhitespace("a  b\n\tc")
	assert.Equal(t, "a b c", norm)
	require.Len(t, offsets, len(norm))
}
