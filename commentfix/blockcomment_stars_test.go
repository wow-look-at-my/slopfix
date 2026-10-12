package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A block whose continuation lines open with stars keeps that run. The
// prose carries none of the stars, and the render writes the same run back.
func TestADoubleStarBlockKeepsItsRun(t *testing.T) {
	shape, prose, ok := readBlock([]string{"/*", "** one", "**", "** two", "*/"})
	assert.True(t, ok)
	assert.Equal(t, "**", shape.stars)
	assert.Equal(t, []string{"one", "", "two"}, prose)
	assert.Equal(t, []string{"/* one", "**", "** two */"}, shape.render(prose))
}

// A single star stays the usual layout.
func TestASingleStarBlockKeepsItsLayout(t *testing.T) {
	shape, prose, ok := readBlock([]string{"\t/*", "\t * one", "\t * two", "\t */"})
	assert.True(t, ok)
	assert.Equal(t, "*", shape.stars)
	assert.Equal(t, []string{"\t/* one", "\t * two */"}, shape.render(prose))
}
