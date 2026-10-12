package cardinal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A numbered list orders its items. Cutting the marker leaves a stray period.
func TestAListMarkerIsNotACount(t *testing.T) {
	assert.Empty(t, texts("//   1. __.PKGDEF (export data)", Comment))
	assert.Empty(t, texts("# 2) the object entries", Comment))
	assert.Equal(t, []string{"3"}, texts("// it reads 3 entries", Comment))
}

// A byte layout written as code names the size each field takes.
func TestACallArgumentOrAnOperandIsNotACount(t *testing.T) {
	assert.Empty(t, texts("//   Header: Magic(8) + Fingerprint(8) + Flags(4) + Offsets(NBlk*4)", Comment))
	assert.Equal(t, []string{"4"}, texts("// it holds (4) parts", Comment))
}
