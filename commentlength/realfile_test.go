package commentlength

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commentspan reports a finding in each of these, and this rule has to report
// one too. Where it does not the repair never runs, and nothing clears the
// build.
//
// Almost every one sits in a single package. That is the shape of a file the
// walk drops whole, rather than a measure that is off by a little.
var commentspanReports = map[string]int{
	"noworkloss/auditedroutes.go": 30,
	"noworkloss/fsverb.go":        50,
	"noworkloss/gitroutes.go":     9,
	"noworkloss/gitverb.go":       264,
	"noworkloss/hook.go":          72,
	"noworkloss/interpreters.go":  115,
	"noworkloss/preserve.go":      100,
	"noworkloss/reach.go":         49,
	"noworkloss/routes.go":        168,
	"noworkloss/segment.go":       367,
	"commentnumbers/numbers.go":   146,
}

// A file the grammar cannot parse yields no finding at all. That is right for a
// tree mid-edit and wrong for committed source: the rule then answers clean, and
// every comment in the file goes unjudged in silence.
func TestEveryRealFileParses(t *testing.T) {
	for name := range commentspanReports {
		path := filepath.Join("..", name)
		src, err := os.ReadFile(path)
		require.NoError(t, err)

		_, ok := treeBlocks(languageFor(path), string(src))
		assert.True(t, ok, "%s does not parse, so the rule reports nothing for it", name)
	}
}

func TestTheRuleReportsWhatCommentspanReports(t *testing.T) {
	var report strings.Builder
	for name, line := range commentspanReports {
		path := filepath.Join("..", name)
		src, err := os.ReadFile(path)
		require.NoError(t, err)

		_, parses := treeBlocks(languageFor(path), string(src))
		found := false
		for _, b := range blocks(path, string(src)) {
			if _, over := judge(b); over && b.start+1 == line {
				found = true
			}
		}
		fmt.Fprintf(&report, "%s:%d parses=%v reported=%v blocks=%d\n",
			name, line, parses, found, len(blocks(path, string(src))))
		assert.True(t, found, "%s:%d is a commentspan finding this rule misses", name, line)
	}
	writeReport(t, report.String())
}

// reportPath names the file the diagnostic lands in. The build prints thousands
// of lines, so a finding written among them is a finding nobody reads.
const reportPath = "parity-report.txt"

func writeReport(t *testing.T, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), reportPath), []byte(body), 0o644))
}
