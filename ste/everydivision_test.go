package ste_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/ste"
)

// Each sentence meets a division that declines it: a gapped list with no
// repeated preposition, a cut inside a compound noun, a bare verb. This happens
// after a comma. The sentence still divides under the cap, so no decline leaves
// it long.
func TestADivisionThatDeclinesLeavesNoSentenceOverTheCap(t *testing.T) {
	for _, in := range []string{
		"The loader reads the settings file and then the cache directory, a lookup table, and an index file, and it writes every result back into the same output folder when the whole run ends today.",
		"The release job uploads the visibility test report and the runner label audit log to the shared artifact store before the final publish step starts on the hosted runner pool.",
		"A feature added to the registry after the rule was written must default to blocked, refuse the open default, and stay that way for every running job on every host in the fleet.",
		"The cache keeps entries for an hour, each with a size limit and a tag, and the index lists every entry that the server has stored since the last full rebuild of the whole table.",
		"Uncommitted work is a different loss than unpushed work -- it is not in git at all -- so it has to be reported even when every commit is on the remote and every branch is clean.",
	} {
		out := ste.Fix(in)
		for _, f := range ste.Check(out, 1) {
			assert.NotEqual(t, ste.IDSentenceCap, f.ID, "%q left a long sentence: %q", in, out)
		}
		assert.NotEqual(t, in, out, "the sentence was not divided")
	}
}
