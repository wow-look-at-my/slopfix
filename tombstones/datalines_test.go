package tombstones

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A run of aligned columns is data, as a table is in a document. The cap does
// not count it, so no finding names it and the repair keeps every line.
func TestTheCapDoesNotCountAnAlignedRun(t *testing.T) {
	var src strings.Builder
	src.WriteString("package main\n\n")
	for _, name := range []string{"input", "header", "body", "cache", "index", "table", "queue", "limit",
		"total", "buffer", "socket", "stream", "record", "schema", "token", "value"} {
		src.WriteString("//   " + name + "    the loop reads it\n")
	}
	src.WriteString("func main() {}\n")

	blocks := AddedBlocks("main.go", src.String())
	assert.Empty(t, Find(blocks, DefaultMaxCommentLines))
	repair := Fix("main.go", src.String(), DefaultMaxCommentLines)
	assert.Equal(t, src.String(), repair.Text)
}

// Prose under an aligned run still counts, so a run with too much prose is a finding.
func TestTheCapCountsTheProseBesideAnAlignedRun(t *testing.T) {
	var src strings.Builder
	src.WriteString("package main\n\n")
	for range DefaultMaxCommentLines + 2 {
		src.WriteString("// The loop reads each value from the input and adds it to the total.\n")
	}
	src.WriteString("//\n//   input    the loop reads it\n")
	src.WriteString("func main() {}\n")

	assert.NotEmpty(t, Find(AddedBlocks("main.go", src.String()), DefaultMaxCommentLines))
}
