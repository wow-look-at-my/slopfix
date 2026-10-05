package english_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/english"
	"github.com/wow-look-at-my/slopfix/ste"
)

func TestCommaNever(t *testing.T) {
	cases := []struct{ in, out string }{
		{"The list names the fork, never its own tree.", "The list names the fork, not its own tree."},
		{"It is a finding, Never a pass.", "It is a finding, not a pass."},
		{"It never fails, and it never waits.", "It never fails, and it never waits."},
		{"Write `a, never b` for it.", "Write `a, never b` for it."},
		{`The owner wrote "a, never b" in the log.`, `The owner wrote "a, never b" in the log.`},
		{"A set, nevertheless, holds.", "A set, nevertheless, holds."},
		// Real sentences: a noun phrase or a participle after "never" takes "not".
		{"The harvested row aborts the wait, never the turn.", "The harvested row aborts the wait, not the turn."},
		{"The bonus is set, never stacked, so this also wipes an earlier score.", "The bonus is set, not stacked, so this also wipes an earlier score."},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			assert.Equal(t, c.out, english.FixCommaNever(c.in))
			assert.Equal(t, c.in != c.out, len(english.CheckCommaNever(c.in, 1)) > 0, "the check and the repair disagree")
			assert.Empty(t, english.CheckCommaNever(english.FixCommaNever(c.in), 1))
		})
	}
}

// Real sentences where a verb follows "never". ", not" before a verb is no
// English, so the text stays and the finding asks for a rewrite by hand.
func TestCommaNeverBeforeAVerbStays(t *testing.T) {
	for _, in := range []string{
		"When true grok never prompts, never gates repo-local configs, and does no trust check.",
		"With prepaid credits but the rule not yet known (None), never warn; it resolves on the next billing fetch.",
		"An auto-allowed command must reach the user prompt, never silently auto-allow it.",
		"Between the two they can only cause a redundant reload later, never leave the cache ahead of the loaded state.",
	} {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, in, english.FixCommaNever(in))
			findings := english.CheckCommaNever(in, 1)
			if assert.NotEmpty(t, findings) {
				assert.Equal(t, english.FixNeverByHand, findings[0].Fix)
				assert.True(t, ste.ByHand(findings[0].Fix))
			}
		})
	}
}
