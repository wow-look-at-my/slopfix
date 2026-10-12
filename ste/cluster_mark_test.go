package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// github-state-mirror docs/ghclient.md: a "+" or a "/" between nouns joins
// multiple runs, so neither sentence holds a cluster to rewrite. Fix
// leaves both as written.
func TestAMarkBetweenNounsEndsTheRun(t *testing.T) {
	for _, s := range []string{
		"GitHub REST + GraphQL API client.",
		"It holds the cache for GitHub App JWT / installation-token minting.",
	} {
		assert.Equal(t, s, FixSelected(s, func(id string) bool { return id == IDNounCluster }), s)
	}
}
