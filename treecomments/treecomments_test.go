package treecomments

import (
	"testing"

	ts "github.com/wow-look-at-my/go-tree-sitter"

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

// A support test asks the extension. Loading the grammar to answer meant a
// caller that only wanted to skip a file it cannot read decoded a parse table,
// and got a panic where the generate step had not run.
func TestSupportedDoesNotLoadTheGrammar(t *testing.T) {
	loaded := false
	restore := grammars[".probe"]
	grammars[".probe"] = func() *ts.Language {
		loaded = true
		return nil
	}
	t.Cleanup(func() {
		if restore == nil {
			delete(grammars, ".probe")
			return
		}
		grammars[".probe"] = restore
	})

	assert.True(t, Supported("a.probe"))
	assert.False(t, loaded, "the table stays on disk until a parse needs it")
	assert.False(t, Supported("a.unknown"))
}
