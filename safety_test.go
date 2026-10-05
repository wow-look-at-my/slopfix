package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/ste"
)

// The doc comment the length repair deleted. Its heading carries the invariant
// for the sentence under it, and the repair cut the block to the heading alone.
func TestTheLengthRepairDoesNotDeleteADocBody(t *testing.T) {
	src := "/// # Safety\n" +
		"/// The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs.\n" +
		"fn f() {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "f.rs", Rules: []slopfix.Rule{slopfix.RuleComments}}).Text
	assert.Contains(t, out, "The caller must ensure", "the doc body was deleted:\n%s", out)
	assert.Contains(t, out, "no other thread writes through it", "the invariant clause was deleted:\n%s", out)
}

// The sentence split that invented a fragment. The tail became a sentence of
// its own with a pronoun subject it never had: "It while this function runs."
func TestALongDocSentenceDoesNotSplitIntoAFragment(t *testing.T) {
	src := "/// # Safety\n" +
		"/// The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs.\n" +
		"fn f() {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "f.rs", Rules: []slopfix.Rule{slopfix.RuleSTE}}).Text
	assert.NotContains(t, out, "It while this function", "the split invented a sentence:\n%s", out)
	assert.Contains(t, out, "no other thread writes through it", "the clause was cut:\n%s", out)
	// Every part must read as a sentence. "the pointer stays. Valid for reads
	// of n bytes" is a copula cut ahead of the adjective it links.
	for _, s := range ste.Sentences(strings.ReplaceAll(ste.Masked(out), "/// ", "")) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		assert.True(t, ste.StandsAlone(s), "a fragment was left: %q\n%s", s, out)
	}
}
