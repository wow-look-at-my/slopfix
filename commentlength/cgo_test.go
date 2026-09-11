package commentlength

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A cgo preamble is C source, so the rule must not measure or cut it.
const cgoSrc = `// Package p wraps a C library.
package p

// #include "compile.h"
// #include <stdlib.h>
import "C"

func Free() { C.free(nil) }
`

func TestFixLeavesACgoPreambleAlone(t *testing.T) {
	out, changed := Fix("p.go", cgoSrc)
	assert.False(t, changed)
	assert.Equal(t, cgoSrc, out)
}

func TestCheckReportsNoCgoPreamble(t *testing.T) {
	for _, hit := range Check("p.go", cgoSrc) {
		assert.NotContains(t, hit.Sentence, "#include")
	}
}

func TestAGroupedImportIsStillMeasured(t *testing.T) {
	src := `package p

// #include <stdlib.h>
import (
	"C"
)
`
	assert.NotEmpty(t, blocks("p.go", src), "a grouped import carries no preamble, so the run is ordinary prose")
}

// A block inside the character budget but over on lines keeps every word: the
// budget's own width is where it goes, and the floor is what allows that.
func TestAWrapOverTheLineCountIsLaidOutRatherThanCut(t *testing.T) {
	src := "// DefaultGLSLVersion is the GLSL version spirv-cross targets when Options\n// leaves GLSLVersion empty.\nconst DefaultGLSLVersion = 450\n"
	out, changed := Fix("p.go", src)
	assert.True(t, changed)
	assert.Contains(t, out, "leaves GLSLVersion empty.")
	assert.Empty(t, Check("p.go", out))
}

// A repair that keeps the line count is still a repair. Comparing counts rather
// than text discarded it, and the character half of the rule is what it answers,
// so the block stayed a finding that no run could clear.
func TestARepairThatKeepsItsLineCountIsStillApplied(t *testing.T) {
	src := "// Run does the thing, and it says so at a length no single line holds inside the budget it must meet, because the words past the floor are what the character half of the rule counts. So the cut lands here.\nfunc Run() {}\n"
	out, changed := Fix("p.go", src)
	assert.True(t, changed, "one line in, one line out, and shorter")
	assert.NotEqual(t, src, out)
	assert.Empty(t, Check("p.go", out))
}

// Laying a block out rescues only what the budget already holds. Prose past it
// is cut mid-sentence rather than left: a run that no honest cut reaches would
// otherwise stay a finding forever, which no automatic pass can clear.
func TestProsePastTheBudgetIsForceFitted(t *testing.T) {
	long := "// Foo names a thing, and then it says a great deal more about that thing, at such length that no width lays it out inside the budget it must meet.\n// A second sentence carries on well past the point.\nconst Foo = 1\n"
	out, changed := Fix("p.go", long)
	assert.True(t, changed)
	assert.NotEqual(t, long, out)
	assert.Contains(t, out, "// Foo names a thing", "the opening survives")
	assert.Empty(t, Check("p.go", out))
}
