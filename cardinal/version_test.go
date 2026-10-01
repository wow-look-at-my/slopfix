package cardinal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A version literal names a release. No edit to the code makes it stale, so it
// is never a count.
func TestAVersionLiteralIsNotACount(t *testing.T) {
	for _, text := range []string{
		"module: vN.0.0 for the major version of path, so v0.0.0 for a path with no",
		"require github.com/wow-look-at-my/foo v0.0.0 // branch=v1",
		"It needs release 1.2.3 or later.",
		"The fork reports v1.27.0-cosmo here.",
		"It pins v2.1.",
	} {
		assert.Empty(t, texts(text, Comment), text)
	}
}

// The negative control: a count beside a version is still a count.
func TestACountBesideAVersionIsStillACount(t *testing.T) {
	assert.Equal(t, []string{"three"}, texts("v0.0.0 carries three fields", Comment))
}
