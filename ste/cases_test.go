package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/table"
)

// What the repair writes for a sentence is stated in rules/ste-sentence.xml,
// where somebody adding a case edits no Go. Each case runs as its own subtest,
// so a failure names the sentence rather than a line number.
func TestEverySentenceCaseHolds(t *testing.T) {
	loaded, err := table.Load(rules.FS, "ste")
	require.NoError(t, err)
	require.NotEmpty(t, loaded.Cases)

	for _, c := range loaded.Cases {
		t.Run(c.Test, func(t *testing.T) {
			assert.Equal(t, c.Expect, ste.Fix(c.Test))
		})
	}
}
