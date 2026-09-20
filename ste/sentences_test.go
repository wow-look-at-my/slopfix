package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/ste"
)

// What the repair writes for a sentence is stated in rules/ste-sentence.xml,
// where somebody adding a case edits no Go. Each test runs as its own subtest,
// so a failure names the sentence rather than a line number.
func TestEverySentenceTestHolds(t *testing.T) {
	cases, err := rules.Tests("ste")
	require.NoError(t, err)
	require.NotEmpty(t, cases)

	for _, c := range cases {
		t.Run(c.In, func(t *testing.T) {
			got := ste.Fix(c.In)
			if c.Out == "" {
				assert.NotEqual(t, c.In, got, "the repair left the sentence alone")
				return
			}
			assert.Equal(t, c.Out, got)
		})
	}
}
