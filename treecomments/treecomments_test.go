package treecomments

import (
	"crypto/sha256"
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
// the proxy holds a grammar whose tables never load. a single such grammar
// must not end the calling toolchain.
func TestANamedGrammarWithoutTablesReadsNoFile(t *testing.T) {
	t.Serial()
	grammars[".tableless"] = func() *ts.Language { return nil }
	t.Cleanup(func() { delete(grammars, ".tableless") })

	assert.Nil(t, languageFor("x.tableless"),
		"a named grammar without tables must answer nil, never the bash fallback")
	assert.Nil(t, Extract("x.tableless", "# a comment\n"),
		"Extract must read nothing rather than panic")
}

// The bash fallback is for a hash-comment file the table does not name.
// Sending a named-but-tableless file there would read Go with a shell grammar.
func TestAHashCommentFileFallsBackToBash(t *testing.T) {
	for _, name := range []string{"Makefile", "Dockerfile", "rules.mk", "suite.dats", "app.ini"} {
		require.NotNil(t, languageFor(name), "%s takes the bash fallback", name)
	}
	assert.Len(t, Extract("Makefile", "# build everything\nall:\n\tgo build\n"), 1)
}

// Bash reads `a&#0;b` as `a &` and then a comment. In a dats fixture that text
// is XML, and a repair that cut it broke the suite. A fallback file keeps only
// the comments that open their line.
func TestAFallbackFileKeepsOnlyWholeLineComments(t *testing.T) {
	src := "tests:\n\t# the suite\n\t- desc: x\n\t  inputs:\n\t\tfiles:\n\t\t\tnul.xml: |\n\t\t\t\t<r>a&#0;b</r>\n"
	got := Extract("suite.dats", src)
	require.Len(t, got, 1)
	assert.Equal(t, "# the suite", got[0].Text)
	assert.Len(t, Extract("run.sh", "echo a&#0;b\n"), 1, "a shell script keeps its trailing comment")
}

// A file of unknown syntax is not read at all. Read as shell, a GLSL
// `#version` directive and a CSS `#id` selector are comments, and a repair
// then cuts code the compiler needs.
func TestAnUnknownExtensionIsNotRead(t *testing.T) {
	glsl := "#version 450\n// a comment\nvoid main() {}\n"
	css := "#bar { color: red; }\n#title { font-weight: 600; }\n"
	for name, src := range map[string]string{"shader.frag": glsl, "shader.comp": glsl, "app.css": css, "x.unknownext": "# text\n"} {
		assert.Nil(t, languageFor(name), "%s has no grammar", name)
		assert.Empty(t, Extract(name, src), "%s yields no comment", name)
		assert.Empty(t, Runs(name, src), "%s yields no run", name)
	}
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

// Every rule on a file asks for the same comments, so a second Extract reuses
// the parse. A caller that changes its copy cannot change the next caller's.
func TestExtractParsesTheSameSourceOnce(t *testing.T) {
	src := "package p\n\n// Cached explains the cache test.\nfunc Cached() {}\n"
	first := Extract("cache.go", src)
	require.Len(t, first, 1)
	key := extractKey{language: languageFor("cache.go"), sum: sha256.Sum256([]byte(src))}
	_, ok := extracted.get(key)
	require.True(t, ok, "the first Extract must remember its parse")

	first[0].Text = "changed by the caller"
	assert.Equal(t, []string{"// Cached explains the cache test."}, texts(Extract("cache.go", src)))
	assert.Empty(t, Extract("cache.go", "package p\n"), "a different source is a different parse")
}

func TestTheExtractCacheDropsItsOldestEntry(t *testing.T) {
	c := &extractCache{byKey: map[extractKey][]Comment{}}
	for i := range extractCacheSize + 1 {
		c.put(extractKey{sum: [sha256.Size]byte{byte(i), byte(i >> 8)}}, nil)
	}
	assert.Len(t, c.byKey, extractCacheSize)
	_, ok := c.get(extractKey{})
	assert.False(t, ok, "the oldest entry goes first")
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
	t.Serial()
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

// A grammar answers nil until its generate step has run, and a rule then reads
// no comment from the languages it covers. Missing is how a caller says so
// instead of reporting a clean file nobody parsed.
func TestMissingNamesTheGrammarsWithoutTables(t *testing.T) {
	// This suite runs from a checkout, where the generate step has run.
	assert.Empty(t, Missing(), "a generated tree owes nothing")

	for _, name := range grammarNames {
		load, ok := ready[name]
		require.True(t, ok, "%s has no readiness answer", name)
		assert.True(t, load(), "%s reports its table", name)
	}
}

// Every grammar the extension map routes to must have a readiness answer.
func TestEveryGrammarIsNamed(t *testing.T) {
	assert.Len(t, ready, len(grammarNames))
}
