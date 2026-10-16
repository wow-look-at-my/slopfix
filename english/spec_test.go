package english

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

func checkTable(body string) error {
	return xmlcheck.Run(Spec(), xmlcheck.File{Name: "english.xml", Raw: []byte(`<english>` + body + `</english>`)})
}

func TestTheShippedTableIsValid(t *testing.T) {
	require.NoError(t, xmlcheck.Run(Spec(), xmlcheck.File{Name: "english.xml", Raw: englishXML}))
}

func TestTheTableFailsOnInvalidContent(t *testing.T) {
	for name, c := range map[string]struct{ body, want string }{
		"a sound entry": {`<drop word="just" test="It just fails"/>`, ""},
		"two drops differing in case": {
			`<drop word="Just" test="It just fails"/><drop word="just" test="It just fails"/>`, "repeats the one"},
		"two sensitive drops differing in case": {
			`<drop word="Just" test="It Just fails" case="sensitive"/><drop word="just" test="It just fails" case="sensitive"/>`, ""},
		"a drop test that does not fire": {`<drop word="just" test="It really fails"/>`, "is matched by nothing"},
		"a drop with child tests of one input": {
			`<drop word="just" test="It just fails"><test in="It JUST fails" out="It fails"/></drop>`, "repeats the one"},
		"a pattern with no expectation": {`<pattern match="a+" replace="b" test="aa"/>`, "needs a expect"},
		"a pattern naming a missing group": {`<pattern match="(a)" replace="$2" test="a" expect="b"/>`, "names the group"},
		"a pattern that does not compile": {`<pattern match="(" replace="b" test="(" expect="b"/>`, "does not compile"},
		"a shape writing a capture nothing makes": {
			`<shape id="s" match="the one {noun:n}" replace="{x}" test="the one cat" expect="cat"/>`, "no term of the match captures"},
		"a rewrite to itself": {`<rewrite from="once" to="ONCE" test="once"/>`, "rewritten to itself"},
		"a phrase behind its part": {
			`<rewrite from="in order" to="to" test="in order"/><rewrite from="in order to" to="to" test="in order to"/>`, "never fires"},
		"a drop behind its part": {
			`<drop word="very" test="very"/><drop word="very much" test="very much"/>`, "never fires"},
		"an expect on a drop": {`<drop word="just" test="It just fails" expect="It fails"/>`, `no attribute "expect"`},
		"a surface outside the three": {`<drop word="just" test="just" where="margin"/>`, "is not one of"},
		"a subject outside the two": {`<pattern match="a" replace="b" test="a" expect="b" subject="after"/>`, "is not one of"},
		"a case value outside the two": {`<drop word="just" test="just" case="yes"/>`, `is not "sensitive" or "insensitive"`},
		"a root test no entry matches": {`<drop word="just" test="just"/><test in="It never fails" out="It fails"/>`, "matched by no entry"},
		"a root test that leaves its input": {`<drop word="just" test="just"/><test in="It never fails" out="It never fails"/>`, ""},
		"two entries answering one input differently": {
			`<drop word="just" test="a just"><test in="It just fails" out="It fails"/></drop>` +
				`<drop word="fails" test="a fails"><test in="It just fails" out="It just"/></drop>`, "expects a different result"},
	} {
		t.Run(name, func(t *testing.T) {
			err := checkTable(c.body)
			if c.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

// A replacement takes the opening case of the phrase it replaces, in the
// middle of a line as at its start.
func TestAReplacementKeepsTheCaseOfWhatItReplaces(t *testing.T) {
	out, took := replaceWordN("It ran. Prior to that it waited.", "prior to", "before", false)
	assert.Equal(t, "It ran. Before that it waited.", out)
	assert.Equal(t, 1, took)
	out, _ = replaceWordN("It ran prior to that.", "prior to", "before", false)
	assert.Equal(t, "It ran before that.", out)
}

// A sensitive entry leaves text whose case differs.
func TestASensitiveEntryMatchesAsWritten(t *testing.T) {
	out, took := replaceWordN("It Just fails", "just", "", true)
	assert.Equal(t, "It Just fails", out)
	assert.Zero(t, took)
	out, took = replaceWordN("It just fails", "just", "", true)
	assert.Equal(t, "It  fails", out)
	assert.Equal(t, 1, took)
	out, took = replaceWordN("It Just fails", "just", "", false)
	assert.Equal(t, "It  fails", out)
	assert.Equal(t, 1, took)
}

func TestNoShippedPatternIsSensitive(t *testing.T) {
	for _, p := range Patterns() {
		assert.False(t, p.Sensitive, "<pattern match=%q> reads case, and the shipped table is insensitive", p.Match)
	}
}
