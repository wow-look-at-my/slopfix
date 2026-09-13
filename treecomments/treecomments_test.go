package treecomments

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ts "github.com/wow-look-at-my/go-tree-sitter"
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

// A module zip carries no generated file, so a consumer resolving slopfix from
// the proxy holds a grammar whose tables never load. One such grammar must not
// end the calling toolchain.
func TestANamedGrammarWithoutTablesReadsNoFile(t *testing.T) {
	grammars[".tableless"] = func() *ts.Language { return nil }
	t.Cleanup(func() { delete(grammars, ".tableless") })

	assert.Nil(t, languageFor("x.tableless"),
		"a named grammar without tables must answer nil, never the bash fallback")
	assert.Nil(t, Extract("x.tableless", "# a comment\n"),
		"Extract must read nothing rather than panic")
}

// The bash fallback is for an extension the table does not name. Sending a
// named-but-tableless file there would read Go with a shell grammar.
func TestAnUnknownExtensionStillFallsBackToBash(t *testing.T) {
	require.NotNil(t, languageFor("Makefile.unknownext"),
		"an unnamed extension takes the bash fallback")
}

func TestAnAbsentGrammarIsNamedOnceForEachExtension(t *testing.T) {
	t.Cleanup(func() { reported.Delete(".saidonce") })

	reportMissingGrammar("first.saidonce")
	_, seen := reported.Load(".saidonce")
	assert.True(t, seen, "the first report records the extension")

	reportMissingGrammar("second.saidonce")
	assert.NotPanics(t, func() { reportMissingGrammar("third.saidonce") },
		"a repeat report is a no-op, not a second line per file")
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
