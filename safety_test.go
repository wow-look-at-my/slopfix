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
	// Every part of the doc body must read as a sentence. "the pointer stays.
	var body string
	for _, line := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "/// The caller") {
			body = strings.TrimSpace(strings.TrimPrefix(t, "///"))
		}
	}
	if body != "" {
		sentences := ste.Sentences(body)
		assert.GreaterOrEqual(t, len(sentences), 2, "the over-cap sentence was left whole:\n%s", out)
		for _, s := range sentences {
			assert.True(t, ste.StandsAlone(strings.TrimSpace(s)), "a fragment was left: %q\n%s", s, out)
		}
	}
}

// A long doc-comment sentence the fixer used to leave standing. It has no main
// clause of its own, so the colon before its list is the boundary the division
// takes. The head closes, and the list it introduces stays whole.
func TestAColonEndsALongDocSentenceBeforeItsList(t *testing.T) {
	src := "/// Every flag a request may carry. An allowlist rather than a deny-list\n" +
		"/// because the flags that matter are the ones that turn a read into a write:\n" +
		"/// `-X POST`, `--method`, `--field`, `--input`.\n" +
		"const ALLOWED_FLAGS: &[&str] = &[\n" +
		"    \"--json\",\n" +
		"];\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "ci_host.rs", Rules: []slopfix.Rule{slopfix.RuleSTE}}).Text
	assert.Contains(t, out, "into a write.\n", "the sentence was not divided at its colon:\n%s", out)
	assert.Contains(t, out, "`-X POST`, `--method`, `--field`, `--input`", "the list was cut:\n%s", out)
}

// A long doc-comment sentence that opens on a participle and holds no main
// clause. The comma before its reason is the boundary the division takes, so
// neither half runs past the cap.
func TestACommaEndsALongFragmentDocSentence(t *testing.T) {
	src := "/// Compiled under `cfg(test)` off Linux as well, because the emitted argv IS the contract (the option order is what makes a later bind win, and what keeps the CI host-worker fd an option rather than a program argument) and an ordering only one host can assert is one that regresses quietly everywhere else.\n" +
		"fn f() {}\n"
	out := slopfix.Fix(slopfix.Request{Content: src, Path: "jail.rs", Rules: []slopfix.Rule{slopfix.RuleSTE}}).Text
	assert.Contains(t, out, "off Linux as well. Because the emitted argv IS the contract", "the sentence was not divided at its comma:\n%s", out)
}
