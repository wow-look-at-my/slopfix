package commentfix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An indented run inside a doc comment is a code block: godoc renders it
// verbatim and a reader reads it as a table. Reflowing one welds its rows
// together, and cutting one drops rows that carry the only description of a
// record.
func indentedBlock(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "indented_block.golden"))
	require.NoError(t, err)
	return string(src)
}

func TestAnIndentedCodeBlockKeepsEveryRow(t *testing.T) {
	out := Fix("putback.go", indentedBlock(t)).Text
	for _, row := range []string{
		"//\tptbL the original parent directory, relative to the volume root",
		"//\tptbN the original name, which differs from the name in the trash when",
		"//\t     something was already called that",
	} {
		assert.Contains(t, out, row, "an indented row is a code block line and survives whole")
	}
}

func TestAnIndentedCodeBlockIsNotReflowed(t *testing.T) {
	out := FixLengthText(t, indentedBlock(t))
	require.NotEmpty(t, out)
	assert.NotContains(t, out, "root ptbN", "two rows joined into one line")
}

// FixLengthText runs the length repair and answers what it wrote.
func FixLengthText(t *testing.T, src string) string {
	t.Helper()
	out, _ := FixLength("putback.go", src)
	if strings.TrimSpace(out) == "" {
		return src
	}
	return out
}
