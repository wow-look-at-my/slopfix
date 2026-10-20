package table_test

import _ "embed"

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/table"
)

// folder is a rules folder standing in for rules/. A case states the file it is
// about rather than depending on what the repository's own tables say.
func folder(files map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, body := range files {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

//go:embed testdata/numbers-file.xml
var numbersFile string

// A file declares its consumer, and Load keeps that consumer's entries alone.
func TestLoadKeepsTheTargetsEntries(t *testing.T) {
	other := testLoadKeepsTheTargetsEntriesXml
	loaded, err := table.Load(folder(map[string]string{"a.xml": numbersFile, "b.xml": other}), "numbers")
	require.NoError(t, err)

	assert.Len(t, loaded.Rewrites, 1)
	assert.Len(t, loaded.Patterns, 1)
	assert.Len(t, loaded.Rephrasings, 1)
	assert.Len(t, loaded.Classes, 1)
	assert.Len(t, loaded.Tests, 1)
	assert.Empty(t, loaded.Drops, "another consumer's entries stay with it")
}

// A list holds a string on each line, with no test per item.
func TestAListHoldsOneItemOnEachLine(t *testing.T) {
	body := testAListHoldsOneItemOnEachLineXml
	loaded, err := table.Load(folder(map[string]string{"a.xml": body}), "words")
	require.NoError(t, err)
	assert.Equal(t, []string{"abate decrease", "abandon go stop"}, loaded.List("banned"))
	assert.Panics(t, func() { loaded.List("missing") })
}

// A list that reads nothing, or a name used twice, fails the load.
func TestABrokenListFailsTheLoad(t *testing.T) {
	empty := `<?xml version="1.0" encoding="UTF-8"?><rules for="words"><list name="banned">  </list></rules>`
	_, err := table.Load(folder(map[string]string{"a.xml": empty}), "words")
	assert.ErrorContains(t, err, "holds no item")

	twice := `<?xml version="1.0" encoding="UTF-8"?><rules for="words"><list name="banned">a</list><list name="banned">b</list></rules>`
	_, err = table.Load(folder(map[string]string{"a.xml": twice}), "words")
	assert.ErrorContains(t, err, "already defined")

	unnamed := `<?xml version="1.0" encoding="UTF-8"?><rules for="words"><list>a</list></rules>`
	_, err = table.Load(folder(map[string]string{"a.xml": unnamed}), "words")
	assert.ErrorContains(t, err, `required attribute "name" is missing`)
}

// The folder is read in file name order. That is what lets a longer phrase in
// an earlier file beat the shorter phrase inside it, which a later file holds.
func TestTheFolderIsReadInFileNameOrder(t *testing.T) {
	first := testTheFolderIsReadInFileNameOrderXml
	second := testTheFolderIsReadInFileNameOrderXml2
	loaded, err := table.Load(folder(map[string]string{"b-later.xml": second, "a-first.xml": first}), "numbers")
	require.NoError(t, err)

	require.Len(t, loaded.Rewrites, 2)
	assert.Equal(t, "one or more", loaded.Rewrites[0].From)
}

// A pattern is compiled as the folder loads, so its match is a regexp the
// entry's own replacement expands against.
func TestALoadedPatternRewritesItsMatch(t *testing.T) {
	loaded, err := table.Load(folder(map[string]string{"a.xml": numbersFile}), "numbers")
	require.NoError(t, err)

	require.Len(t, loaded.Patterns, 1)
	assert.Equal(t, "It reserves a single slot", loaded.Patterns[0].Replace("It reserves one slot"))
	assert.Equal(t, "nothing here", loaded.Patterns[0].Replace("nothing here"))
}

// A class the folder declares is indexed, which is what a rephrase match reads.
func TestALoadedClassIsIndexed(t *testing.T) {
	loaded, err := table.Load(folder(map[string]string{"a.xml": numbersFile}), "numbers")
	require.NoError(t, err)
	assert.True(t, loaded.Lexicon().Is("slot", "noun"))
}

// Every way a file can be unusable says which entry, and says it at the read
// rather than by matching nothing later.
func TestLoadRefusesWhatCannotFire(t *testing.T) {
	for name, body := range map[string]string{
		"a pattern that does not compile": `<rules for="numbers"><pattern id="bad" match="one(" to="x">
<test in="one("/></pattern></rules>`,
		"a rephrase whose match is malformed": `<rules for="numbers"><rephrase id="bad" match="{" to="x">
<test in="{"/></rephrase></rules>`,
		"an entry carrying no test": `<rules for="numbers"><rewrite id="bare" from="a" to="b"/></rules>`,
		"an entry carrying no id":   `<rules for="numbers"><rewrite from="a" to="b"><test in="a"/></rewrite></rules>`,
		"a root naming no consumer": `<rules><rewrite id="x" from="a" to="b"><test in="a"/></rewrite></rules>`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := table.Load(folder(map[string]string{"a.xml": `<?xml version="1.0" encoding="UTF-8"?>` + body}), "numbers")
			assert.Error(t, err)
		})
	}
}

// An id names an entry across the whole folder, so another file may not take
// an id an earlier file used. A failure would otherwise name an entry the
// reader cannot find.
func TestLoadRefusesADuplicateIDAcrossFiles(t *testing.T) {
	again := testLoadRefusesADuplicateIDAcrossFilesXml
	_, err := table.Load(folder(map[string]string{"a.xml": numbersFile, "b.xml": again}), "numbers")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "once")
}

// A consumer nothing declares is a wiring mistake rather than an empty table,
// and a caller reading an empty table would silently repair nothing.
func TestLoadRefusesATargetNothingDeclares(t *testing.T) {
	_, err := table.Load(folder(map[string]string{"a.xml": numbersFile}), "ste")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ste")
}

func TestAppliesToReadsTheSurface(t *testing.T) {
	assert.True(t, table.AppliesTo("", "comment"))
	assert.True(t, table.AppliesTo("both", "message"))
	assert.True(t, table.AppliesTo("message", "message"))
	assert.False(t, table.AppliesTo("message", "comment"))
}

//go:embed testdata/test-load-keeps-the-targets-entries.xml
var testLoadKeepsTheTargetsEntriesXml string

//go:embed testdata/test-a-list-holds-one-item-on-each-line.xml
var testAListHoldsOneItemOnEachLineXml string

//go:embed testdata/test-the-folder-is-read-in-file-name-order.xml
var testTheFolderIsReadInFileNameOrderXml string

//go:embed testdata/test-the-folder-is-read-in-file-name-order-2.xml
var testTheFolderIsReadInFileNameOrderXml2 string

//go:embed testdata/test-load-refuses-a-duplicate-id-across-files.xml
var testLoadRefusesADuplicateIDAcrossFilesXml string
