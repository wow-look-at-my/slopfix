package tombstones

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Specimens caught in the wild rather than written for the test. Each pairs
// with the text that replaced it, so the control proves the rule accepts the
// repair as well as rejecting the original. An invented tombstone only proves
// the rule matches what its author had in mind.

// A purge comment in a setup script, beside the code removing a hook
const purgeNarration = `# fix-what-you-found.ts moved from Stop to MessageDisplay: a Stop hook runs
# after the message has streamed, so refusing cannot unsend it -- the user reads
# the punt, then a near-identical retype that writes the tell down again and
# trips the guard a second time. The FILE stays (MessageDisplay runs it), so the
# loop above cannot express this; only the stale Stop entry goes. Without it a
# HOME set up by an older payload both refuses and annotates on one message.
TMP="$(mktemp)"
`

// What replaced it. Both load-bearing facts survive, and the narrative is gone.
const purgeRepair = `# The loop above purges by FILE. This file stays, so only its stale Stop entry
# goes: an older HOME otherwise refuses and annotates on the same message.
TMP="$(mktemp)"
`

func TestAPurgeCommentNarratingItsOwnMigrationIsATombstone(t *testing.T) {
	repair := Fix("setup/bootstrap.sh", purgeNarration, DefaultMaxCommentLines)

	require.NotEmpty(t, append(repair.Removed, tellsOf(repair)...),
		"the specimen must be caught, or this fixture proves nothing")
}

func TestThePurgeCommentsRepairIsAccepted(t *testing.T) {
	repair := Fix("setup/bootstrap.sh", purgeRepair, DefaultMaxCommentLines)

	assert.False(t, repair.Changed, "the repair must survive the rule it was written to satisfy")
	assert.Empty(t, repair.Kept)
	assert.Empty(t, repair.Removed)
}

// tellsOf names what a repair reported but could not excise, so a test can ask
// whether a specimen was caught at all.
func tellsOf(r Repair) []string {
	var out []string
	for _, hit := range r.Kept {
		out = append(out, hit.Tell)
	}
	return out
}
