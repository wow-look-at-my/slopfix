package table_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/table"
)

// folder is a rules folder standing in for rules/, so a case states the file
// it is about rather than depending on what the repository's own tables say.
func folder(files map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for name, body := range files {
		out[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return out
}

const oneRule = `<?xml version="1.0" encoding="UTF-8"?>
<rules for="numbers">
  <rewrite from="once" to="a single time" test="It hashes once" expect="It hashes a single time"/>
  <pattern match="(?i)\bone\s+([a-z])" to="a single ${1}" test="It reserves one slot" expect="It reserves a single slot"/>
  <case test="It reserves one slot" expect="It reserves a single slot"/>
</rules>
`

// A file declares its consumer, and Load keeps that consumer's entries alone.
func TestLoadKeepsTheTargetsEntries(t *testing.T) {
	other := `<?xml version="1.0" encoding="UTF-8"?>
<rules for="english">
  <drop word="basically" test="It basically fails"/>
</rules>
`
	loaded, err := table.Load(folder(map[string]string{"a.xml": oneRule, "b.xml": other}), "numbers")
	require.NoError(t, err)

	assert.Len(t, loaded.Rewrites, 1)
	assert.Len(t, loaded.Patterns, 1)
	assert.Len(t, loaded.Cases, 1)
	assert.Empty(t, loaded.Drops, "another consumer's entries stay with it")
}

func TestTheFolderIsReadInFileNameOrder(t *testing.T) {
	first := `<?xml version="1.0" encoding="UTF-8"?>
<rules for="numbers"><rewrite from="one or more" to="any number of" test="one or more" expect="any number of"/></rules>
`
	second := `<?xml version="1.0" encoding="UTF-8"?>
<rules for="numbers"><rewrite from="one" to="a single" test="one" expect="a single"/></rules>
`
	loaded, err := table.Load(folder(map[string]string{"b-later.xml": second, "a-first.xml": first}), "numbers")
	require.NoError(t, err)

	require.Len(t, loaded.Rewrites, 2)
	assert.Equal(t, "one or more", loaded.Rewrites[0].From)
}

// A pattern is compiled as the folder loads, so its match is a regexp the
// entry's own replacement expands against.
func TestALoadedPatternRewritesItsMatch(t *testing.T) {
	loaded, err := table.Load(folder(map[string]string{"a.xml": oneRule}), "numbers")
	require.NoError(t, err)

	require.Len(t, loaded.Patterns, 1)
	assert.Equal(t, "It reserves a single slot", loaded.Patterns[0].Replace("It reserves one slot"))
	assert.Equal(t, "nothing here", loaded.Patterns[0].Replace("nothing here"))
}

// A match that does not compile is a broken table, and it says which entry.
func TestLoadRefusesAPatternThatDoesNotCompile(t *testing.T) {
	broken := `<?xml version="1.0" encoding="UTF-8"?>
<rules for="numbers"><pattern match="one(" to="x" test="one(" expect="x"/></rules>
`
	_, err := table.Load(folder(map[string]string{"a.xml": broken}), "numbers")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "one(")
}

// A root with no for= names no consumer, so nothing can tell whose entry it is.
func TestLoadRefusesAFileNamingNoConsumer(t *testing.T) {
	_, err := table.Load(folder(map[string]string{
		"a.xml": "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rules><drop word=\"just\" test=\"It just fails\"/></rules>\n",
	}), "numbers")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "for=")
}

// A consumer nothing declares is a wiring mistake rather than an empty table,
// and a caller reading an empty table would silently repair nothing.
func TestLoadRefusesATargetNothingDeclares(t *testing.T) {
	_, err := table.Load(folder(map[string]string{"a.xml": oneRule}), "ste")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ste")
}

func TestAppliesToReadsTheSurface(t *testing.T) {
	assert.True(t, table.AppliesTo("", "comment"))
	assert.True(t, table.AppliesTo("both", "message"))
	assert.True(t, table.AppliesTo("message", "message"))
	assert.False(t, table.AppliesTo("message", "comment"))
}
