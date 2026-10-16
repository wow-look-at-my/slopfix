package table_test

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

// file wraps entries in a rules root for the consumer "numbers".
func file(root, body string) string {
	return `<?xml version="1.1" encoding="UTF-8"?>` + "\n" + `<rules for="numbers"` + root + `>` + body + `</rules>`
}

func load(root, body string) (*table.Table, error) {
	return table.Load(folder(map[string]string{"a.xml": file(root, body)}), "numbers")
}

// Every rules file the binary embeds belongs to a consumer, and every consumer
// loads through the checks. A file that no target reads would never be checked.
func TestEveryEmbeddedRulesFileIsChecked(t *testing.T) {
	paths, err := fs.Glob(rules.FS, "*.xml")
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	targets := map[string]bool{}
	for _, path := range paths {
		raw, err := fs.ReadFile(rules.FS, path)
		require.NoError(t, err)
		root, err := xmlcheck.Parse(path, raw)
		require.NoError(t, err)
		targets[root.Attrs["for"]] = true
	}
	for target := range targets {
		_, err := table.Load(rules.FS, target)
		require.NoError(t, err, "target %s", target)
	}
}

// A pattern ignores case unless its entry or its file says otherwise.
func TestAPatternIgnoresCaseUnlessSensitive(t *testing.T) {
	loose, err := load("", `<pattern id="p" match="\bone\b" to="a single"><test in="It has one" out="It has a single"/></pattern>`)
	require.NoError(t, err)
	assert.Equal(t, "It has a single", loose.Patterns[0].Replace("It has ONE"))
	assert.False(t, loose.Patterns[0].Sensitive)

	exact, err := load(` case="sensitive"`, `<pattern id="p" match="\bone\b" to="a single"><test in="It has one" out="It has a single"/></pattern>`)
	require.NoError(t, err)
	assert.Equal(t, "It has ONE", exact.Patterns[0].Replace("It has ONE"))
	assert.True(t, exact.Patterns[0].Sensitive)

	override, err := load(` case="sensitive"`, `<pattern id="p" match="\bone\b" to="a single" case="insensitive"><test in="It has one" out="It has a single"/></pattern>`)
	require.NoError(t, err)
	assert.False(t, override.Patterns[0].Sensitive, "an entry overrides the file")
}

// A sensitive rephrase literal compares as written.
func TestASensitiveRephraseLiteralComparesAsWritten(t *testing.T) {
	body := `<class name="noun" open="true"/><rephrase id="r" match="Only one" to="a single" case="sensitive"><test in="Only one here" out="A single here"/></rephrase>`
	loaded, err := load("", body)
	require.NoError(t, err)
	lex := loaded.Lexicon()
	assert.Equal(t, "A single here", table.Rephrasings(lex, nil, loaded.Rephrasings, "Only one here"))
	assert.Equal(t, "only one here", table.Rephrasings(lex, nil, loaded.Rephrasings, "only one here"))
}

func TestARephraseIgnoresCaseByDefault(t *testing.T) {
	body := `<rephrase id="r" match="only one" to="a single"><test in="Only one here" out="A single here"/></rephrase>`
	loaded, err := load("", body)
	require.NoError(t, err)
	assert.Equal(t, "A single here", table.Rephrasings(loaded.Lexicon(), nil, loaded.Rephrasings, "Only one here"))
}

// Each defect below stops the load, and the error says which.
func TestLoadRefusesInvalidContent(t *testing.T) {
	test := func(in, out string) string { return `<test in="` + in + `" out="` + out + `"/>` }
	for name, c := range map[string]struct{ root, body, want string }{
		"two drops that differ in case": {"", `<drop id="a" word="Just">` + test("It just fails", "It fails") + `</drop><drop id="b" word="just">` + test("It just fails", "It fails") + `</drop>`, "repeats the one"},
		"two sensitive drops that differ in case are fine": {` case="sensitive"`, `<drop id="a" word="Just">` + test("It Just fails", "It fails") + `</drop><drop id="b" word="just">` + test("It just fails", "It fails") + `</drop>`, ""},
		"two tests with one input": {"", `<drop id="a" word="just">` + test("It just fails", "It fails") + test("it JUST fails", "it fails") + `</drop>`, "repeats the one"},
		"two entries expecting two results": {"", `<drop id="a" word="just">` + test("It just fails", "It fails") + `</drop><drop id="b" word="fails">` + test("It just fails", "It just") + `</drop>`, "expects a different result"},
		"a test its entry does not match": {"", `<drop id="a" word="just">` + test("It really fails", "It fails") + `</drop>`, "is matched by nothing"},
		"a rephrase test its match does not fit": {"", `<rephrase id="r" match="only one" to="a single">` + test("Only two here", "A single here") + `</rephrase>`, "is matched by nothing"},
		"a pattern that does not compile": {"", `<pattern id="p" match="(" to="x">` + test("(", "x") + `</pattern>`, "error parsing regexp"},
		"a pattern naming a missing group": {"", `<pattern id="p" match="(one)" to="$2">` + test("one", "x") + `</pattern>`, "names the group"},
		"a rephrase writing a capture nothing makes": {"", `<rephrase id="r" match="only one" to="{n}">` + test("only one", "") + `</rephrase>`, "no term of the match captures"},
		"a normalize writing a space": {"", `<normalize id="n" from="a" to="b c">` + test("a", "b c") + `</normalize>`, "holds a space"},
		"a rewrite to itself": {"", `<rewrite id="w" from="once" to="Once">` + test("once", "Once") + `</rewrite>`, "rewritten to itself"},
		"a longer phrase behind its part": {"", `<rewrite id="a" from="one" to="a single">` + test("one", "a single") + `</rewrite><rewrite id="b" from="one or more" to="any">` + test("one or more", "any") + `</rewrite>`, "never fires"},
		"an unknown attribute": {"", `<drop id="a" word="just" color="red">` + test("just", "") + `</drop>`, `no attribute "color"`},
		"a case value outside the two": {"", `<drop id="a" word="just" case="maybe">` + test("just", "") + `</drop>`, `is not "sensitive" or "insensitive"`},
		"two classes of one name": {"", `<class name="n" words="a"/><class name="n" words="b"/>`, "repeats the one"},
		"a normalize that takes case": {"", `<normalize id="n" from="a" to="b" case="sensitive">` + test("a", "b") + `</normalize>`, "does not take case="},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := load(c.root, c.body)
			if c.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

// ste reads a rewrite through a lowercased lookup, so its rewrites take no case.
func TestSteRewritesTakeNoCaseAttribute(t *testing.T) {
	body := `<rewrite id="a" from="can't" to="cannot" case="sensitive"><test in="It can't run"/></rewrite>`
	_, err := table.Load(folder(map[string]string{"a.xml": `<rules for="ste">` + body + `</rules>`}), "ste")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not take case=")
}
