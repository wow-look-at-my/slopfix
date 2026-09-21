package slopfix_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wow-look-at-my/slopfix"
)

// repaired answers what every rule makes of a Go file.
func repaired(t *testing.T, src string) string {
	t.Helper()
	return slopfix.Fix(slopfix.Request{Path: "probe.go", Content: src}).Text
}

// Rewriting it to "a single" makes a sentence that says something else, and
// often makes no sentence at all: "a rule file that names no schema is a
// single nothing checks".
func TestAPronounIsNotATally(t *testing.T) {
	src := "package p\n\n// probe reports whether a file that names no schema is one nothing checks.\nfunc probe() bool {\n\treturn true\n}\n"
	out := repaired(t, src)
	assert.NotContains(t, out, "a single nothing checks")
	assert.Contains(t, out, "is one nothing checks")
}

// A cardinal that says how many things the code takes is the meaning of the
// sentence. Cutting it leaves prose that reads as if the number never mattered.
func TestACountThatSaysHowManyIsKept(t *testing.T) {
	src := "package p\n\n// probe adds three numbers and multiplies the two sums together.\nfunc probe(a, b, c int) int {\n\tx := a + b\n\ty := b + c\n\treturn x * y\n}\n"
	out := repaired(t, src)
	assert.Contains(t, out, "three numbers")
	assert.Contains(t, out, "the two sums")
}
