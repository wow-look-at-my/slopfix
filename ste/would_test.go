package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
)

// "would" takes the present tense of its verb group. "Will" turns a condition
// into a promise, so no repair writes it.
func TestWouldTakesThePresentTense(t *testing.T) {
	for in, want := range map[string]string{
		"Without the cache the build would fail.":    "Without the cache the build fails.",
		"A cached tree would be faster.":             "A cached tree is faster.",
		"The old runner would have dropped the job.": "The old runner drops the job.",
		"The gate would not accept the token.":       "The gate does not accept the token.",
		"Would the gate accept the token?":           "Does the gate accept the token?",
		"Would it be faster?":                        "Is it faster?",
		"The runners would also fail.":               "The runners also fail.",
	} {
		got := ste.FixSelected(in, func(id string) bool { return id == ste.IDModal })
		assert.Equal(t, want, got, in)
		assert.NotContains(t, got, "will", in)
	}
}

// "wouldn't" expands to what it says. Alone, the contraction repair writes
// "would not". With ste/modal in the same fix, the group takes the present tense.
func TestWouldntExpandsToWouldNot(t *testing.T) {
	assert.Equal(t, "It would not attach.", ste.FixSelected("It wouldn't attach.", func(id string) bool { return id == ste.IDContraction }))
	assert.Equal(t, "It does not attach.", ste.Fix("It wouldn't attach."))
}
