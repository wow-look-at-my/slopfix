package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// A block over the volume cap sits above code long enough that the length rule
// alone never cuts it. The fix still brings it under the cap.
func TestFixCutsABlockBackUnderTheVolumeCap(t *testing.T) {
	var src strings.Builder
	src.WriteString("package p\n\n")
	for range 20 {
		src.WriteString("// Run reads the queue and writes each entry out, then it waits for the next one to come in.\n")
	}
	src.WriteString("func Run() {\n")
	for range 40 {
		src.WriteString("\tstep(theFirstArgumentOfTheCall, theSecondArgumentOfTheCall, theThirdArgument)\n")
	}
	src.WriteString("}\n")
	assert.Empty(t, commentfix.CheckLength("a.go", src.String()), "only the volume cap judges this block")

	repair := slopfix.Fix(slopfix.Request{Content: src.String(), Path: "a.go", MaxCommentLines: tombstones.DefaultMaxCommentLines})
	comments := 0
	for _, line := range strings.Split(repair.Text, "\n") {
		if strings.HasPrefix(line, "//") {
			comments++
		}
	}
	assert.LessOrEqual(t, comments, tombstones.DefaultMaxCommentLines, repair.Text)
	assert.Contains(t, repair.Text, "func Run() {\n\tstep(theFirstArgumentOfTheCall", "the code is untouched")
}
