package cardinal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A command argument joined by colons is a name, so no digit in it is a count.
// A ratio of bare digits is still read.
func TestAJoinedArgumentIsNotACount(t *testing.T) {
	assert.Empty(t, Find("make radv-cmp ARGS='shader.comp -d 0:0:ssbo'", Comment))
	assert.NotEmpty(t, Find("the ratio is 2:1 here", Comment))
}
