package tombstones

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix/edit"
)

// scopedSrc is a comment block whose second row holds a hedged-time phrase.
const scopedSrc = "package p\n\n// Retry opens the dial.\n// It currently retries once on a refused dial.\n// The caller owns the timeout.\nfunc Retry() {}\n"

// rowScope bounds the scope to one row of scopedSrc, as a fork scope does when
// the fork wrote that row and no other.
func rowScope(t *testing.T, row string) edit.Scope {
	at := strings.Index(scopedSrc, row)
	require.GreaterOrEqual(t, at, 0)
	return edit.Within(at, at+len(row))
}

// A scope that holds one row of a block refuses the block rewrite, so the
// phrase on that row is cut from the row alone.
func TestAPhraseIsCutFromTheOneRowAScopeHolds(t *testing.T) {
	row := "// It currently retries once on a refused dial."
	repair := FixIn("p.go", scopedSrc, 0, rowScope(t, row))

	assert.Equal(t, strings.Replace(scopedSrc, row, "// It retries once on a refused dial.", 1), repair.Text)
	assert.Positive(t, repair.Rewrites)
	assert.Empty(t, repair.Kept)
}

// A phrase no edit can reach is reported on its own row, not on the first row
// of the block that holds it.
func TestAPhraseNoEditReachesIsReportedOnItsRow(t *testing.T) {
	repair := FixIn("p.go", scopedSrc, 0, edit.Nowhere())

	assert.Equal(t, scopedSrc, repair.Text)
	require.Len(t, repair.Kept, 1)
	assert.Equal(t, "tombstones/hedged-time", repair.Kept[0].ID)
	assert.Equal(t, 4, repair.Kept[0].LineNo)
}
