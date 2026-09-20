package table_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/table"
)

// folder is a rules folder standing in for rules/, so a case states the file it
// is about rather than depending on what the repository's own tables say.
func folder(files map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, body := range files {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

const numbersFile = `<?xml version="1.0" encoding="UTF-8"?>
<rules for="numbers">
  <rewrite id="once" from="once" to="a single time">
    <test in="It hashes once" out="It hashes a single time"/>
  </rewrite>
  <pattern id="one-noun" match="(?i)\bone\s+([a-z])" to="a single ${1}">
    <test in="It reserves one slot" out="It reserves a single slot"/>
  </pattern>
  <class name="noun" open="true"/>
  <rephrase id="only-one" match="only one" to="a single">
    <test in="It locks only one" out="It locks a single"/>
  </rephrase>
  <test in="It reserves one slot" out="It reserves a single slot"/>
</rules>
`

// A file declares its consumer, and Load keeps that consumer's entries alone.
func TestLoadKeepsTheTargetsEntries(t *testing.T) {
	other := `<?xml version="1.0" encoding="UTF-8"?>
<rules for="english">
  <drop id="basically" word="basically"><test in="It basically fails"/></drop>
</rules>
`
	loaded, err := table.Load(folder(map[string]string{"a.xml": numbersFile, "b.xml": other}), "numbers")
	require.NoError(t, err)

	assert.Len(t, loaded.Rewrites, 1)
	assert.Len(t, loaded.Patterns, 1)
	assert.Len(t, loaded.Rephrasings, 1)
	assert.Len(t, loaded.Classes, 1)
	assert.Len(t, loaded.Tests, 1)
	assert.Empty(t, loaded.Drops, "another consumer's entries stay with it")
}

// The folder is read in file name order. That is what lets a longer phrase in
// an earlier file beat the shorter phrase inside it, which a later file holds.
func TestTheFolderIsReadInFileNameOrder(t *testing.T) {
	first := `<?xml version="1.0" encoding="UTF-8"?>
<rules for="numbers"><rewrite id="one-or-more" from="one or more" to="any number of">
<test in="one or more" out="any number of"/></rewrite></rules>
`
	second := `<?xml version="1.0" encoding="UTF-8"?>
<rules for="numbers"><rewrite id="one" from="one" to="a single">
<test in="one" out="a single"/></rewrite></rules>
`
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
// an id an earlier file used: a failure would otherwise name an entry the
// reader cannot find.
func TestLoadRefusesADuplicateIDAcrossFiles(t *testing.T) {
	again := `<?xml version="1.0" encoding="UTF-8"?>
<rules for="numbers"><rewrite id="once" from="once" to="a single time">
<test in="It hashes once" out="It hashes a single time"/></rewrite></rules>
`
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
