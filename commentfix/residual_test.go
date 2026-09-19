package commentfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/commentfix"
)

// unreachable carries a number no paragraph rewrite reaches.
const unreachable = `package p

/*
Layout:
  3 columns
*/
func f() {}
`

// The claim this rule now makes: a repaired file reports nothing.
func TestFixLeavesNothingForCheckToFind(t *testing.T) {
	for name, src := range map[string]string{
		"a block the rewrite declines": unreachable,
		"a number beside code":         "package p\n\nvar x = 1 // 3 slots\n",
		"a plain sentence":             "package p\n\n// It holds 4 keys.\nfunc f() {}\n",
		"a number the table says":      "package p\n\n// It reserves one slot.\nfunc f() {}\n",
		"a directive and a number":     "package p\n\n//go:generate tool -n 3\n// It holds 7 keys.\nfunc f() {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			fixed := commentfix.Fix("f.go", src)
			assert.Empty(t, commentfix.Check("f.go", fixed.Text),
				"the repair left a finding in:\n%s", fixed.Text)
		})
	}
}

// The repair must not reach past a comment. Code is what the compiler reads,
// and a rewrite that touches it is a rewrite nobody can accept.
func TestFixLeavesCodeAlone(t *testing.T) {
	fixed := commentfix.Fix("f.go", unreachable)
	require.Contains(t, fixed.Text, "package p")
	require.Contains(t, fixed.Text, "func f() {}")
	assert.NotContains(t, fixed.Text, "3 columns")
}

// A file the repair cannot improve is left exactly as it is.
func TestFixWritesNothingWithoutAFinding(t *testing.T) {
	src := "package p\n\n// It holds the keys.\nfunc f() {}\n"
	fixed := commentfix.Fix("f.go", src)
	assert.False(t, fixed.Changed)
	assert.Equal(t, src, fixed.Text)
}

// Repairing an already repaired file changes nothing further.
func TestFixIsSettled(t *testing.T) {
	once := commentfix.Fix("f.go", unreachable)
	twice := commentfix.Fix("f.go", once.Text)
	assert.Equal(t, once.Text, twice.Text)
	assert.False(t, twice.Changed)
}
