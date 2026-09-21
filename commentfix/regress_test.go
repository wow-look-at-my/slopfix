package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The repair must never hand back a file with the code gone. Whatever it decides
// about a block, the declaration under it is not the repair's to remove.
func TestTheRepairNeverEmptiesAFile(t *testing.T) {
	for name, src := range map[string]string{
		"rust inner doc": innerDoc,
		"rust outer doc": essay("///") + "const P: i32 = 1;\n",
		"rust line":      essay("//") + "const P: i32 = 1;\n",
	} {
		fixed, changed := FixLength("x.rs", src)
		if !changed {
			continue
		}
		assert.NotEmpty(t, fixed, "%s: the repair emptied the file", name)
	}
}
