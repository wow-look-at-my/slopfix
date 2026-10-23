package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A sentence about a lock divides under the cap whatever its quotes, code and
// coordinated names do to the parse.
func TestLockSentencesDivideUnderTheCap(t *testing.T) {
	for _, in := range []string{
		"Every job takes a slot in \"<state dir>.pool/\": a serial number from the index file, taken under flock on that file, names a slot file that its owner holds flock(LOCK_EX) on for as long as it waits or holds.",
		"main() holds the \"disk\" lock for every sync-tool run except --help and --dump-config, so only one sync-tool uses the disk at a time.",
	} {
		out := Fix(in)
		assert.Zero(t, overCap(out, SentenceWordCap), out)
	}
}
