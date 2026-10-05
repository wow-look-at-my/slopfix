package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// The doc comment the length repair deleted. Its heading carries the invariant
// for the sentence under it, and the repair cut the block to the heading alone.
func TestTheLengthRepairDoesNotDeleteADocBody(t *testing.T) {
	src := "/// # Safety\n" +
		"/// The caller must ensure that the pointer stays valid for reads of n bytes for the whole call, and that no other thread writes through it while this function runs.\n" +
		"fn f() {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "f.rs", Rules: []slopfix.Rule{slopfix.RuleComments}}).Text
	assert.Contains(t, out, "The caller must ensure", "the doc body was deleted:\n%s", out)
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
}

// A block of one long sentence with no full stop reaches the volume cap. The
// repair cannot cut it at a thought, so it condenses the block to the cap.
func TestTheVolumeCapReachesItsCapWhenNoThoughtCutFits(t *testing.T) {
	comment := "// the block continues one sentence with no full stop anywhere in it\n"
	src := "package main\n\n" + strings.Repeat(comment, 20) + "func main() {}\n"
	req := slopfix.Request{Content: src, Path: "x.go", Rules: []slopfix.Rule{slopfix.RuleTombstones}, MaxCommentLines: tombstones.DefaultMaxCommentLines}
	out := slopfix.Fix(req).Text
	assert.NotContains(t, findingIDs(slopfix.CheckContent("x.go", out)), tombstones.IDVolume, "the cap did not reach itself:\n%s", out)
}
