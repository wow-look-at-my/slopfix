package treecomments

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cgoFile = `// Package p wraps a C library.
package p

// #include "compile.h"
// #include <stdlib.h>
import "C"

// Free releases the buffers the parser took.
func Free() {}
`

func texts(comments []Comment) []string {
	out := make([]string, 0, len(comments))
	for _, c := range comments {
		out = append(out, c.Text)
	}
	return out
}

func TestExtractSkipsTheCgoPreamble(t *testing.T) {
	got := texts(Extract("p.go", cgoFile))
	assert.Equal(t, []string{
		"// Package p wraps a C library.",
		"// Free releases the buffers the parser took.",
	}, got)
}

func TestACgoPreambleIsOneRunAwayFromBeingRewrapped(t *testing.T) {
	for _, run := range Runs("p.go", cgoFile) {
		for _, c := range run {
			assert.NotContains(t, c.Text, "#include")
		}
	}
}

func TestABlankLineEndsThePreamble(t *testing.T) {
	src := `package p

// This sentence is prose and stays a comment.

// #include <stdlib.h>
import "C"
`
	got := texts(Extract("p.go", src))
	assert.Equal(t, []string{"// This sentence is prose and stays a comment."}, got)
}

func TestAGroupedImportCarriesNoPreamble(t *testing.T) {
	src := `package p

// This sentence is prose, because a grouped import has no preamble.
import (
	"C"
)
`
	got := texts(Extract("p.go", src))
	assert.Equal(t, []string{"// This sentence is prose, because a grouped import has no preamble."}, got)
}

func TestAFileWithoutCgoKeepsEveryComment(t *testing.T) {
	src := `// Package p does no cgo.
package p

// Free does nothing.
func Free() {}
`
	require.Len(t, Extract("p.go", src), 2)
}
