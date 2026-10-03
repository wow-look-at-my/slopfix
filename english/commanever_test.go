package english_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/english"
)

func TestCommaNever(t *testing.T) {
	cases := []struct{ in, out string }{
		{"The list names the fork, never its own tree.", "The list names the fork, not its own tree."},
		{"It is a finding, Never a pass.", "It is a finding, not a pass."},
		{"It never fails, and it never waits.", "It never fails, and it never waits."},
		{"Write `a, never b` for it.", "Write `a, never b` for it."},
		{`The owner wrote "a, never b" in the log.`, `The owner wrote "a, never b" in the log.`},
		{"A set, nevertheless, holds.", "A set, nevertheless, holds."},
		{"Never retry, never re-run, never widen a timeout.", "Never retry, never re-run, never widen a timeout."},
		{"It runs. A failure is fixed, never concealed.", "It runs. A failure is fixed, not concealed."},
		{"It never fails. A failure is fixed, never concealed.", "It never fails. A failure is fixed, not concealed."},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			assert.Equal(t, c.out, english.FixCommaNever(c.in))
			assert.Equal(t, c.in != c.out, len(english.CheckCommaNever(c.in, 1)) > 0, "the check and the repair disagree")
			assert.Empty(t, english.CheckCommaNever(english.FixCommaNever(c.in), 1))
		})
	}
}
