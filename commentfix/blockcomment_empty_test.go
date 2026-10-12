package commentfix

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A trim can keep no line of a block comment. The render then answers nothing
// instead of reading a first line that is not there.
func TestRenderOfNoProseIsNothing(t *testing.T) {
	shape := blockShape{indent: "\t", opener: "/*"}
	assert.Nil(t, shape.render(nil))
	assert.Nil(t, shape.render([]string{}))
	assert.Equal(t, []string{"\t/* one line */"}, shape.render([]string{"one line"}))
}
