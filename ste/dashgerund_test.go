package ste

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/syntax"
)

func TestAGerundThatHidesASilentEReadsItsBaseForm(t *testing.T) {
	for gerund, base := range map[string]string{"using": "use", "writing": "write", "seeing": "see", "being": "be", "running": "run"} {
		assert.Equal(t, base, baseFromGerund(gerund), gerund)
	}
}

// The long sentence of the bashfs profiling package comment. It ends on
// "-- using hyperfine", an -ing form after a dash. Every part it divides into
// is a sentence under the cap, and the -ing form becomes a finite verb.
func TestAnIngFormAfterADashDividesUnderTheCap(t *testing.T) {
	in := "Then it gains a guarded block that does nothing on a normal run but, when the environment variable BASHFS_PROFILE_SCRIPT=1 is set, benchmarks the parts of the script that bashfs owns -- the startup integrity check and each embedded file's extraction -- using hyperfine, then exits without running the user's body."
	out := Fix(in)
	masked := mask(in)
	s := syntax.Parse(masked, opaque(in, masked))
	for _, sentence := range Sentences(checkMask(out)) {
		assert.LessOrEqual(t, WordCount(sentence), SentenceWordCap, "%s\n%s\n%s\n%s", out, s.Outline(), traceDivisions(s, in), traceForce(in))
	}
	assert.Contains(t, out, "It uses hyperfine, then exits without running the user's body.", out)
}
