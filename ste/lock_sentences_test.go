package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A sentence about a lock divides under the cap whatever its quotes, code and
// coordinated names do to the parse.
func TestLockSentencesDivideUnderTheCap(t *testing.T) {
	for _, in := range []string{
		"Every request takes a ticket in \"<lock file>.queue/\": a sequence number from the counter file, taken under flock on that file, names a ticket file that its owner holds flock(LOCK_EX) on for as long as it waits or holds.",
		"main() holds the \"gpu\" lock for every splat-vulkan run except --help and --dump-layouts, so only one splat-vulkan uses the GPU at a time.",
	} {
		out := Fix(in)
		assert.Zero(t, overCap(out, SentenceWordCap), out)
	}
}
