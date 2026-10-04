package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnifiedDiffShowsTheChangedLineInContext(t *testing.T) {
	before := "package p\n\n// It holds 3 keys.\nvar a int\n"
	after := "package p\n\n// It holds keys.\nvar a int\n"
	assert.Equal(t, "--- a/x.go\n+++ b/x.go\n"+
		"@@ -1,5 +1,5 @@\n"+
		" package p\n"+
		" \n"+
		"-// It holds 3 keys.\n"+
		"+// It holds keys.\n"+
		" var a int\n"+
		" \n", UnifiedDiff("x.go", before, after))
}

func TestUnifiedDiffOfNoChangeIsEmpty(t *testing.T) {
	assert.Empty(t, UnifiedDiff("x.go", "same\n", "same\n"))
}

// Changes far apart are separate hunks, so the diff does not print the file.
func TestUnifiedDiffSplitsDistantChanges(t *testing.T) {
	before := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	after := "A\nb\nc\nd\ne\nf\ng\nh\ni\nJ\n"
	assert.Equal(t, "--- a/x\n+++ b/x\n"+
		"@@ -1,3 +1,3 @@\n-a\n+A\n b\n c\n"+
		"@@ -8,4 +8,4 @@\n h\n i\n-j\n+J\n \n", UnifiedDiff("x", before, after))
}
