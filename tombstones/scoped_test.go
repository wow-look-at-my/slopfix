package tombstones

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/edit"
)

// scopedSrc is a comment block whose second line holds a hedged-time phrase.
const scopedSrc = "package p\n\n// Retry opens the dial.\n// It currently retries once on a refused dial.\n// The caller owns the timeout.\nfunc Retry() {}\n"

// lineScope bounds the scope to one line of scopedSrc, as a fork scope does
// when the fork wrote that line and no other.
func lineScope(t *testing.T, line string) edit.Scope {
	at := strings.Index(scopedSrc, line)
	require.GreaterOrEqual(t, at, 0)
	return edit.Within(at, at+len(line))
}

// A scope that holds one line of a block refuses the block rewrite. The
// phrase on that line is cut by an edit of that line alone.
func TestAPhraseIsCutInTheOneLineAScopeHolds(t *testing.T) {
	line := "// It currently retries once on a refused dial."
	repair := FixIn("p.go", scopedSrc, 0, lineScope(t, line))

	assert.Equal(t, strings.Replace(scopedSrc, line, "// It retries once on a refused dial.", 1), repair.Text)
	assert.Positive(t, repair.Rewrites)
	assert.Empty(t, repair.Kept)
}

// A phrase no edit can reach is reported on its own line, not on the first
// line of the block that holds it.
func TestAPhraseNoEditReachesIsReportedOnItsLine(t *testing.T) {
	repair := FixIn("p.go", scopedSrc, 0, edit.Nowhere())

	assert.Equal(t, scopedSrc, repair.Text)
	require.Len(t, repair.Kept, 1)
	assert.Equal(t, "tombstones/hedged-time", repair.Kept[0].ID)
	assert.Equal(t, 4, repair.Kept[0].LineNo)
}
