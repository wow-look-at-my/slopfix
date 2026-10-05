package commentfix

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A number that opens a continuation line takes the mark after it up to the
// line above. No line is left holding a lone period.
func TestDeletingANumberThatOpensALineKeepsItsMarkOnTheLineAbove(t *testing.T) {
	for name, src := range map[string]string{
		"aligned": "  /* the controls and miss those\n     two. */\n",
		"starred": "  /* the controls and miss those\n   * two. */\n",
	} {
		t.Run(name, func(t *testing.T) {
			at := strings.Index(src, "two")
			e := closeGap(src, at, at+len("two"))
			got := src[:e.Start] + e.Text + src[e.End:]
			assert.Equal(t, "  /* the controls and miss those. */\n", got)
		})
	}
}
