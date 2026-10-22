package xmlcheck_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

// miniature is a small table that exercises each kind of check, and its schema
// is testdata/miniature.xsd.
func miniature() xmlcheck.Spec {
	return xmlcheck.Spec{
		Schema: miniatureSchema,
		Elems: map[string]xmlcheck.Elem{
			"t": {Case: true},
			"e": {
				Case: true, TestAttr: "probe", ExpectAttr: "expect",
				Keys: []xmlcheck.Key{{Attrs: []string{"word"}, Exact: []string{"where"}}},
			},
			"r": {Keys: []xmlcheck.Key{{Attrs: []string{"name"}, Split: true}}},
		},
		Test:    xmlcheck.TestDef{Elem: "test", In: "in", Out: []string{"out"}},
		Shadows: []xmlcheck.Shadow{{Elem: "e", Attr: "word"}},
		Fires: func(n *xmlcheck.Node, sensitive bool, in string) bool {
			return xmlcheck.HasPhrase(in, n.Attrs["word"], sensitive)
		},
		FiresAny: func(roots []*xmlcheck.Node, in string) bool {
			for _, root := range roots {
				for _, n := range root.Children {
					if n.Name == "e" && xmlcheck.HasPhrase(in, n.Attrs["word"], n.Sensitive()) {
						return true
					}
				}
			}
			return false
		},
	}
}

func run(spec xmlcheck.Spec, body string) error {
	return xmlcheck.Run(spec, xmlcheck.File{Name: "t.xml", Raw: []byte(`<?xml version="1.1" encoding="UTF-8"?>` + "\n" + body)})
}

func TestASoundTablePasses(t *testing.T) {
	require.NoError(t, run(miniature(), soundTable))
}

// Every defect names the file and the line it sits on.
func TestAProblemNamesItsLine(t *testing.T) {
	err := run(miniature(), unknownElement)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "t.xml:4:")
}

func TestTheSchemaRejectsAnUnknownElement(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><bogus/></t>`), `"bogus"`)
}

func TestTheSchemaRejectsAnElementInTheWrongPlace(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="a"><e word="b"/></e></t>`), `"e"`)
}

func TestTheSchemaRejectsAnUnknownAttribute(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" color="red"/></t>`), `unexpected attribute "color"`)
}

func TestTheSchemaRejectsAMissingOrBlankRequiredAttribute(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="  "/></t>`), "does not match pattern")
	assert.ErrorContains(t, run(miniature(), `<t><e/></t>`), `required attribute "word" is missing`)
}

func TestTheSchemaRejectsAValueOutsideItsSet(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" level="medium"/></t>`), "is not one of the allowed values")
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" where="a c"/></t>`), "is not one of the allowed values")
}

func TestTheSchemaRejectsTextWhereNoneBelongs(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="a">stray</e></t>`), "t.xml:2:")
	assert.ErrorContains(t, run(miniature(), `<t><e word="a">stray</e></t>`), "takes no text")
}

// Text belongs to an element whose schema declares it, and an element whose
// schema declares only children and attributes carries none.
func TestTextIsReadWhereTheSchemaDeclaresIt(t *testing.T) {
	require.NoError(t, run(miniature(), `<t><s word="a">body</s></t>`))
	for _, c := range []struct {
		name string
		body string
		err  string
	}{
		{"text in an element of children and attributes", `<t><e word="a">stray</e></t>`, "takes no text"},
		{"text in an element of attributes alone", `<t><f word="a">stray</f></t>`, "expects empty content"},
		{"blanks where no text belongs", "<t><e word=\"a\">\n  </e></t>", ""},
		{"a child where a child belongs", `<t><e word="a"><test in="an alpha" out="x"/></e></t>`, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := run(miniature(), c.body)
			if c.err == "" {
				require.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, c.err)
		})
	}
}

func TestTheSchemaRejectsACaseValueOutsideTheTwo(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t case="Sensitive"/>`), "is not one of the allowed values")
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" case="yes"/></t>`), "is not one of the allowed values")
}

func TestTheSchemaRejectsACaseAttributeOnAnElementThatDeclaresNone(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><test in="a" case="sensitive"/></t>`), `unexpected attribute "case"`)
}

// A schema may declare case= on an element the table does not let an entry flip.
func TestACaseAttributeOnAnElementThatTakesNoneFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><f word="a" case="sensitive"/></t>`), "does not take case=")
}

// The semantic checks read a table whose shape is sound, so a schema defect
// is the only thing reported while one stands.
func TestSemanticChecksWaitForTheSchema(t *testing.T) {
	err := run(miniature(), `<t><e word="a"/><e word="A"/><bogus/></t>`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"bogus"`)
	assert.NotContains(t, err.Error(), "repeats the one")
}

// Each file is validated on its own, so each bad file is named.
func TestEveryFileWithASchemaDefectIsReported(t *testing.T) {
	a := xmlcheck.File{Name: "a.xml", Raw: []byte(`<t><bogus/></t>`)}
	b := xmlcheck.File{Name: "b.xml", Raw: []byte(`<t><e/></t>`)}
	err := xmlcheck.Run(miniature(), a, b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.xml:1:")
	assert.Contains(t, err.Error(), "b.xml:1:")
}

// Case does not separate entries unless both are sensitive.
func TestTwoEntriesDifferingInCaseCollide(t *testing.T) {
	err := run(miniature(), `<t><e word="Alpha"/><e word="alpha"/></t>`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repeats the one at t.xml:2")
	assert.Contains(t, err.Error(), "case does not separate them")
}

func TestTwoSensitiveEntriesDifferingInCaseAreDistinct(t *testing.T) {
	require.NoError(t, run(miniature(), `<t case="sensitive"><e word="Alpha"/><e word="alpha"/></t>`))
	assert.ErrorContains(t, run(miniature(), `<t case="sensitive"><e word="alpha"/><e word="alpha"/></t>`), "repeats the one")
}

// An entry that says insensitive under a sensitive root is insensitive, and so is the pair.
func TestAnEntryOverridesTheRootsCase(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t case="sensitive"><e word="Alpha"/><e word="alpha" case="insensitive"/></t>`), "repeats the one")
	require.NoError(t, run(miniature(), `<t case="insensitive"><e word="Alpha" case="sensitive"/><e word="alpha" case="sensitive"/></t>`))
}

func TestTheSameKeyOnAnotherSurfaceIsAnotherEntry(t *testing.T) {
	require.NoError(t, run(miniature(), `<t><e word="a" where="a"/><e word="a" where="b"/></t>`))
}

// An example is its input as written. The example examples that differ only
// in case are probes of what the entry does with case, not one example
// written twice. Only the same text written twice repeats.
func TestATestRepeatsOnlyWhenItIsWrittenTheSameWay(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		err  string
	}{
		{
			name: "one example written twice",
			body: `<t><e word="alpha"><test in="an alpha" out="x"/><test in="an alpha" out="x"/></e></t>`,
			err:  `the test "an alpha" repeats the one at t.xml:2`,
		},
		{
			name: "two examples that differ in case",
			body: `<t><e word="alpha"><test in="an alpha" out="x"/><test in="An ALPHA" out="x"/></e></t>`,
		},
		{
			name: "two examples of one input in two cases, answered in their own case",
			body: `<t><e word="alpha"><test in="an alpha" out="x"/><test in="An ALPHA" out="X"/></e></t>`,
		},
		{
			name: "the case-only pair the numbers table ships",
			body: `<t><e word="ways"><test in="consumed two ways" out="multiple ways"/><test in="consumed Two ways" out="multiple ways"/></e></t>`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := run(miniature(), c.body)
			if c.err == "" {
				require.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, c.err)
		})
	}
}

// A broken table names each defect it holds, and the schema's defect is the
// one reported while the table's shape is unsound.
func TestABrokenTableNamesEveryDefectItHolds(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		err  []string
	}{
		{
			name: "duplicate keys",
			body: `<t><e word="alpha"/><e word="alpha"/></t>`,
			err:  []string{`<e> "alpha" repeats the one at t.xml:2`},
		},
		{
			name: "keys that differ only in case under an insensitive root",
			body: `<t><e word="Alpha"/><e word="alpha"/></t>`,
			err:  []string{"repeats the one at t.xml:2", "case does not separate them"},
		},
		{
			name: "a case-only pair under a sensitive root",
			body: `<t case="sensitive"><e word="Alpha"/><e word="alpha"/></t>`,
		},
		{
			name: "a row its entry never matches",
			body: `<t><e word="alpha"><test in="a beta" out="x"/></e></t>`,
			err:  []string{`the test "a beta" is matched by nothing in its <e>`},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := run(miniature(), c.body)
			if len(c.err) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range c.err {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestTwoSensitiveTestsDifferingInCaseAreDistinct(t *testing.T) {
	require.NoError(t, run(miniature(), `<t case="sensitive"><e word="alpha"><test in="an alpha" out="x"/><test in="An alpha" out="y"/></e></t>`))
}

func TestARootTestAndAnInlineTestOfOneEntryAreOneScope(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="alpha" probe="an alpha" expect="x"><test in="an alpha" out="x"/></e></t>`), "repeats the one")
}

func TestOneInputExpectingTwoResultsFails(t *testing.T) {
	err := run(miniature(), twoResultsForOneInput)
	assert.ErrorContains(t, err, "expects a different result")
}

func TestOneInputExpectingOneResultInTwoPlacesPasses(t *testing.T) {
	require.NoError(t, run(miniature(), oneResultInTwoPlaces))
}

func TestATestItsEntryDoesNotMatchFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="alpha"><test in="a beta" out="x"/></e></t>`), "is matched by nothing in its <e>")
	assert.ErrorContains(t, run(miniature(), `<t><e word="alpha" probe="a beta" expect="x"/></t>`), "is matched by nothing in its <e>")
}

func TestATestThatChangesNothingNeedsNoMatch(t *testing.T) {
	require.NoError(t, run(miniature(), `<t><e word="alpha"><test in="a beta" out="a beta"/></e></t>`))
}

func TestARootTestNoEntryMatchesFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="alpha"/><test in="a beta" out="x"/></t>`), "matched by no entry of the table")
}

func TestAnEntryClaimedByAnEarlierPhraseFails(t *testing.T) {
	err := run(miniature(), `<t><e word="in order"/><e word="in order to"/></t>`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "never fires")
}

func TestALongerPhraseAheadOfItsPartPasses(t *testing.T) {
	require.NoError(t, run(miniature(), `<t><e word="in order to"/><e word="in order"/></t>`))
}

func TestAPhraseInsideAWordIsNotClaimed(t *testing.T) {
	require.NoError(t, run(miniature(), `<t><e word="just"/><e word="adjustment"/></t>`))
}

func TestAnEarlierEntryOnOtherSurfacesClaimsNothing(t *testing.T) {
	require.NoError(t, run(miniature(), `<t><e word="in order" where="a"/><e word="in order to" where="b"/></t>`))
}

func TestTheWrongRootFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<other/>`), `"other"`)
}

func TestAnEntryHookReportsItsProblems(t *testing.T) {
	spec := miniature()
	spec.Entry = func(n *xmlcheck.Node, sensitive bool) []string {
		if n.Attrs["word"] == "bad" {
			return []string{"this entry is no good"}
		}
		return nil
	}
	assert.ErrorContains(t, run(spec, `<t><e word="bad"/></t>`), "this entry is no good")
	require.NoError(t, run(spec, `<t><e word="good"/></t>`))
}

func TestMalformedXMLFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="a"></t>`), "t.xml:")
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" word="b"/></t>`), "t.xml:")
}

func TestEverySemanticProblemIsReportedAtOnce(t *testing.T) {
	err := run(miniature(), `<t><e word="Alpha"/><e word="alpha"/><e word="x"><test in="zzz" out="1"/></e><test in="qqq" out="2"/></t>`)
	require.Error(t, err)
	assert.GreaterOrEqual(t, strings.Count(err.Error(), "t.xml:"), 3)
}

func TestKeysAcrossFilesOfOneTableCollide(t *testing.T) {
	a := xmlcheck.File{Name: "a.xml", Raw: []byte(`<t><e word="alpha"/></t>`)}
	b := xmlcheck.File{Name: "b.xml", Raw: []byte(`<t><e word="ALPHA"/></t>`)}
	err := xmlcheck.Run(miniature(), a, b)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "b.xml:1")
	assert.Contains(t, err.Error(), "a.xml:1")
}

func TestListedKeysSplitOnCommas(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><r name="ls"/><r name="dir, ls"/></t>`), "repeats the one")
	require.NoError(t, run(miniature(), `<t><r name="ls"/><r name="dir"/></t>`))
}

func TestAPatternWithAGroupItLacksFails(t *testing.T) {
	assert.Empty(t, xmlcheck.PatternProblems(`(a)(b)`, `${2} $1 $$`, false))
	assert.NotEmpty(t, xmlcheck.PatternProblems(`(a)`, `$2`, false))
	assert.NotEmpty(t, xmlcheck.PatternProblems(`(a)`, `$1b`, false), "$1b names the group 1b")
	assert.Empty(t, xmlcheck.PatternProblems(`(?P<x>a)`, `${x}`, false))
	assert.NotEmpty(t, xmlcheck.PatternProblems(`a(`, ``, false))
}

func TestAPatternIgnoresCaseUnlessSensitive(t *testing.T) {
	loose, err := xmlcheck.Compile(`abc`, false)
	require.NoError(t, err)
	assert.True(t, loose.MatchString("xABCx"))
	exact, err := xmlcheck.Compile(`abc`, true)
	require.NoError(t, err)
	assert.False(t, exact.MatchString("xABCx"))
}

func TestARewriteToItselfOrAroundItselfFails(t *testing.T) {
	assert.NotEmpty(t, xmlcheck.RewriteProblems("in order to", "In Order To", false))
	assert.Empty(t, xmlcheck.RewriteProblems("in order to", "In Order To", true))
	assert.NotEmpty(t, xmlcheck.RewriteProblems("prior to", "just prior to now", false))
	assert.Empty(t, xmlcheck.RewriteProblems("prior to", "before", false))
}

func TestAnXML11DeclarationReads(t *testing.T) {
	root, err := xmlcheck.Parse("x.xml", []byte(xml11Table))
	require.NoError(t, err)
	require.Len(t, root.Children, 1)
	assert.Equal(t, 3, root.Children[0].Line)
}
