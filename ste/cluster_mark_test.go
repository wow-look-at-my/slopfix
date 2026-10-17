package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// clusterFix runs the noun-cluster repair alone.
func clusterFix(s string) string {
	return FixSelected(s, func(id string) bool { return id == IDNounCluster })
}

// noCluster asserts that no noun-cluster finding is left in s.
func noCluster(t *testing.T, s string) {
	t.Helper()
	for _, f := range Warn(s, 1, false) {
		assert.NotEqual(t, IDNounCluster, f.ID, "%s: %s", s, f.Detail)
	}
}

// A "+" or a "/" between nouns stands for a conjunction. The repair writes it, which divides the run into the
// noun phrases the mark joined.
func TestAMarkBetweenNounsBecomesItsConjunction(t *testing.T) {
	for in, want := range map[string]string{
		"Vendor SOAP + JSON RPC client.":                               "Vendor SOAP and JSON RPC client.",
		"It holds the cache for Acme App JWT / session-token minting.": "It holds the cache for Acme App JWT or session-token minting.",
	} {
		got := clusterFix(in)
		assert.Equal(t, want, got, in)
		noCluster(t, got)
	}
}

// A possessive inside a run names the owner of the nouns after it, and the
// repair writes that owner with "of".
func TestAPossessiveInARunBecomesItsOwner(t *testing.T) {
	in := "The tool reads Acme's llvm clang toolchain build."
	got := clusterFix(in)
	assert.NotEqual(t, in, got)
	assert.Contains(t, got, "Acme")
	assert.Contains(t, got, "build")
	noCluster(t, got)
}
