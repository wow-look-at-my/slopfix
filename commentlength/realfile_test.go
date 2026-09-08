package commentlength

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
)

// commentspan reports these, and this rule has to report them too, or the
// repair never runs there and nothing can clear the build.
//
// The synthetic shapes in parity_test.go pass while these real ones do not, so
// the divergence lives in the measure. This names the file and the line, and
// prints what the judge computed, which is the only way to see the gap.
var commentspanReports = map[string][]int{
	"noworkloss/auditedroutes.go": {30, 41, 62},
	"noworkloss/fsverb.go":        {50, 75},
	"noworkloss/gitroutes.go":     {9, 137, 143, 147},
	"noworkloss/gitverb.go":       {264},
	"noworkloss/hook.go":          {72, 89},
	"noworkloss/interpreters.go":  {115},
	"commentnumbers/numbers.go":   {146},
}

func TestTheRuleReportsWhatCommentspanReports(t *testing.T) {
	for name, wanted := range commentspanReports {
		path := filepath.Join("..", name)
		src, err := os.ReadFile(path)
		require.NoError(t, err)

		reported := set.New[int]()
		for _, b := range blocks(path, string(src)) {
			if _, over := judge(b); over {
				reported.Add(b.start + 1)
			}
		}
		for _, line := range wanted {
			assert.True(t, reported.Contains(line))

		}
	}
}

// explain prints what the judge measured at a line, so a miss names its cause
// rather than only its place.
func explain(path, src string, line int) string {
	for _, b := range blocks(path, src) {
		if b.start+1 != line {
			continue
		}
		lines, chars := measure(prose(b.text))
		return fmt.Sprintf("  block found: comment %d lines / %d chars,"+
			" code %d lines / %d chars, exact=%v",
			lines, chars, b.codeLines, b.codeChars, b.exact)
	}
	return "  no block starts at that line: the comment run was never paired"
}
