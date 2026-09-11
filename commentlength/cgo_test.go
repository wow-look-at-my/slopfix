package commentlength

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A cgo preamble is C source. Tightening it joins one #include onto another and
// the package stops compiling, so the rule must not measure or cut it.
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
