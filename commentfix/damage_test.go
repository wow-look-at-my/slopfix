package commentfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix/commentfix"
)

// An abbreviation's period is not a sentence end. A cut that split there left
// half a sentence behind, ending on "e.g.".
func TestACutDoesNotSplitAtAnAbbreviation(t *testing.T) {
	src := "// It keeps the slot. A reading (nothing consumed, e.g. 42 in total) is hidden.\nvar x int\n"
	repair := fix(t, src)
	assert.Equal(t, "// It keeps the slot.\nvar x int\n", repair.Text)
	assert.Equal(t, []string{"A reading (nothing consumed, e.g. 42 in total) is hidden."}, repair.Removed)

	src = "// It keeps the slot, i.e. the one it reserved. Each shard holds 128.\nvar x int\n"
	repair = fix(t, src)
	assert.Equal(t, "// It keeps the slot, i.e. the one it reserved.\nvar x int\n", repair.Text)
}

// A line indented past its marker is laid out by hand: an aligned list, a
// table, an example. The rewrite leaves it as written and never flows prose into it.
func TestAnIndentedCommentLineKeepsItsLayout(t *testing.T) {
	src := "// Talks to two endpoints:\n" +
		"//   GET /api/me                   -> Me\n" +
		"//   GET /api/cache?scope=mine|all -> CacheResponse\n" +
		"func f() {}\n"
	repair := fix(t, src)
	assert.Equal(t, "// Talks to endpoints:\n"+
		"//   GET /api/me                   -> Me\n"+
		"//   GET /api/cache?scope=mine|all -> CacheResponse\n"+
		"func f() {}\n", repair.Text)
}

// A trailing comment keeps every word when its rewrite grows past the line. A
// wrapped tail once began with the code before it, and the join lost it.
func TestAGrownTrailingCommentKeepsItsTail(t *testing.T) {
	src := "var cases = []int{\n" +
		"\t1, // span 21 → mid band\n" +
		"}\n"
	repair := fix(t, src)
	assert.Contains(t, repair.Text, "mid band", "the tail of the trailing comment was dropped:\n%s", repair.Text)
}

// A value the code compares against is a literal, not a count of what exists.
func TestAComparedValueIsNotACount(t *testing.T) {
	for _, src := range []string{
		"// Rows with used > 0 are shown.\nvar x int\n",
		"// It stops when used <= 0 holds.\nvar x int\n",
		"// A limit >= 100 is refused.\nvar x int\n",
	} {
		assert.Empty(t, commentfix.Check("x.go", header+src), "%q", src)
		assert.False(t, fix(t, src).Changed, "%q", src)
	}
}
