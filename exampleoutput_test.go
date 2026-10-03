package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// The examples a repair once broke in xml-validator. go test compares each
// output block with what the example prints, so a cut turns the test off.
const exampleSource = `package validator_test

import "fmt"

func ExampleParseTree() {
	fmt.Println("r", 1, 1)
	// Output: r 1 1
}

func ExampleLines() {
	fmt.Println(2)
	fmt.Println(3)
	// Output:
	// 2
	// 3
}

func ExampleError() {
	fmt.Printf("validation failed at line %d, column %d\n", 1, 1)
	// Unordered output: validation failed at line 1, column 1
}
`

func TestARepairKeepsExampleOutput(t *testing.T) {
	req := slopfix.Request{Path: "example_test.go", Content: exampleSource, MaxCommentLines: tombstones.DefaultMaxCommentLines}
	assert.Empty(t, slopfix.Report(req).Findings, "an output block is data, not prose")
	assert.Equal(t, exampleSource, slopfix.Fix(req).Text)
}

// Only a test file holds examples. The same comments elsewhere are prose, and every rule reads them.
func TestOnlyATestFileDropsExampleOutput(t *testing.T) {
	texts := func(path string) []string {
		var out []string
		for _, c := range treecomments.Extract(path, exampleSource) {
			out = append(out, c.Text)
		}
		return out
	}
	assert.Empty(t, texts("example_test.go"))
	assert.Equal(t, []string{
		"// Output: r 1 1", "// Output:", "// 2", "// 3",
		"// Unordered output: validation failed at line 1, column 1",
	}, texts("example.go"))
}
