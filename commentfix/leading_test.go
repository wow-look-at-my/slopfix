package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// An empty block comment that leads a table row is an alignment placeholder,
// as runtime's signal tables write them. It documents nothing and is not judged.
func TestACommentLeadingCodeOnItsLineIsNotJudged(t *testing.T) {
	src := strings.Join([]string{
		"package p",
		"",
		"var sigtable = [...]sigTabT{",
		"\t/* */ {0, \"SIGNONE: no trap\"},",
		"\t/* 1 */ {_SigNotify + _SigKill, \"SIGHUP: terminal line hangup\"},",
		"}",
		"",
	}, "\n")
	assert.Empty(t, CheckLength("a.go", src))
	_, changed := FixLength("a.go", src)
	assert.False(t, changed)
}
