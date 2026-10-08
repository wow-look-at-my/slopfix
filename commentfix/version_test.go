package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The number repair leaves a version literal whole, in prose and in a godoc
// code block alike.
func TestTheNumberRepairKeepsAVersionLiteral(t *testing.T) {
	src := "package p\n\n" +
		"// placeholder reports the token of an org module. It is the major\n" +
		"// module: vN.0.0 for the major version of path, so v0.0.0 for a path with no\n" +
		"// major suffix.\n" +
		"func placeholder() {}\n\n" +
		"// Example:\n" +
		"//\n" +
		"//\trequire github.com/wow-look-at-my/foo v0.0.0 // branch=v1\n" +
		"func example() {}\n"
	assert.Empty(t, Check("x.go", src))
	assert.Equal(t, src, Fix("x.go", src).Text)
}
