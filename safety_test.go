package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
)

// The first sentence of a `# Safety` block carries the invariant the caller
// must meet. A sentence no clause boundary divides cleanly stays as written.
// The splitter once invented a sentence from the tail: "It while this function
// runs."
func TestALongDocSentenceStaysAsWrittenWhenNoCleanDivisionExists(t *testing.T) {
	src := "/// # Safety\n" +
		"/// The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs.\n" +
		"pub unsafe extern \"C\" fn f(p: *const u8, n: usize) {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "f.rs", Rules: []slopfix.Rule{slopfix.RuleSTE}}).Text
	assert.NotContains(t, out, "It while this function", "the split invented a sentence:\n%s", out)
	assert.Contains(t, out, "no other thread writes through it while this function runs", "the clause was cut:\n%s", out)
	assert.Contains(t, out, "# Safety", "the marker is gone:\n%s", out)
}

// The length repair cuts a comment back inside its budget, but a doc it cannot
// shorten without losing the point stays as written. It once cut the block to
// its heading alone: "/// # Safety."
func TestADocTheLengthRepairCannotShortenStaysAsWritten(t *testing.T) {
	src := "/// # Safety\n" +
		"/// The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs.\n" +
		"pub unsafe extern \"C\" fn f(p: *const u8, n: usize) {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "f.rs", Rules: []slopfix.Rule{slopfix.RuleComments}}).Text
	assert.Contains(t, out, "The caller must ensure", "the doc body was deleted:\n%s", out)
	assert.Contains(t, out, "no other thread writes through it", "the invariant clause was deleted:\n%s", out)
}
