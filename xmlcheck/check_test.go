package xmlcheck_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

// miniature is a small table that exercises each kind of check.
func miniature() xmlcheck.Spec {
	return xmlcheck.Spec{
		Root: "t",
		Elems: map[string]xmlcheck.Elem{
			"t": {Case: true, Children: []string{"e", "test"}},
			"e": {
				Required: []string{"word"}, Attrs: []string{"level", "where"}, Case: true,
				Children: []string{"test"}, TestAttr: "probe", ExpectAttr: "expect",
				Enum:  map[string][]string{"level": {"low", "high"}, "where": {"a", "b"}},
				Lists: []string{"where"},
				Keys:  []xmlcheck.Key{{Attrs: []string{"word"}, Exact: []string{"where"}}},
			},
			"test": {Required: []string{"in"}, Attrs: []string{"out"}},
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
	require.NoError(t, run(miniature(), `<t>
  <e word="alpha" level="low" where="a b"><test in="an alpha" out="a"/></e>
  <e word="beta" probe="a beta" expect="b"/>
  <test in="an alpha one" out="x"/>
</t>`))
}

// Every defect names the file and the line it sits on.
func TestAProblemNamesItsLine(t *testing.T) {
	err := run(miniature(), `<t>
  <e word="alpha"/>
  <bogus/>
</t>`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "t.xml:4:")
}

func TestAnUnknownElementFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><bogus/></t>`), "<bogus> is not an element")
}

func TestAnElementInTheWrongPlaceFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="a"><e word="b"/></e></t>`), "may not appear inside")
}

func TestAnUnknownAttributeFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" color="red"/></t>`), `no attribute "color"`)
}

func TestAnEmptyRequiredAttributeFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="  "/></t>`), "needs a word that is not empty")
	assert.ErrorContains(t, run(miniature(), `<t><e/></t>`), "needs a word that is not empty")
}

func TestAValueOutsideItsSetFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" level="medium"/></t>`), `level="medium" is not one of`)
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" where="a c"/></t>`), `where="c" is not one of`)
}

func TestTextWhereNoneBelongsFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="a">stray</e></t>`), "may not hold text")
}

func TestACaseValueOutsideTheTwoFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t case="Sensitive"/>`), `is not "sensitive" or "insensitive"`)
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" case="yes"/></t>`), `is not "sensitive" or "insensitive"`)
}

func TestACaseAttributeOnAnElementThatTakesNoneFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><test in="a" case="sensitive"/></t>`), "does not take case=")
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

func TestTwoTestsWithOneInputInOneScopeFail(t *testing.T) {
	err := run(miniature(), `<t><e word="alpha"><test in="an alpha" out="x"/><test in="An ALPHA" out="y"/></e></t>`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `the test "An ALPHA" repeats the one at t.xml:2`)
}

func TestTwoSensitiveTestsDifferingInCaseAreDistinct(t *testing.T) {
	require.NoError(t, run(miniature(), `<t case="sensitive"><e word="alpha"><test in="an alpha" out="x"/><test in="An alpha" out="y"/></e></t>`))
}

func TestARootTestAndAnInlineTestOfOneEntryAreOneScope(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<t><e word="alpha" probe="an alpha" expect="x"><test in="an alpha" out="x"/></e></t>`), "repeats the one")
}

func TestOneInputExpectingTwoResultsFails(t *testing.T) {
	err := run(miniature(), `<t>
  <e word="alpha"><test in="an alpha" out="x"/></e>
  <e word="alp"><test in="an alpha" out="y"/></e>
</t>`)
	assert.ErrorContains(t, err, "expects a different result")
}

func TestOneInputExpectingOneResultInTwoPlacesPasses(t *testing.T) {
	require.NoError(t, run(miniature(), `<t>
  <e word="alpha"><test in="an alpha" out="x"/></e>
  <test in="an alpha" out="x"/>
</t>`))
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

func TestARootThatMatchesAsTypedNeedsTheAttribute(t *testing.T) {
	spec := miniature()
	spec.Sensitive = true
	assert.ErrorContains(t, run(spec, `<t/>`), `says case="sensitive"`)
	require.NoError(t, run(spec, `<t case="sensitive"/>`))
	assert.ErrorContains(t, run(spec, `<t case="sensitive"><e word="a" case="insensitive"/></t>`), `cannot say case="insensitive"`)
}

func TestTheWrongRootFails(t *testing.T) {
	assert.ErrorContains(t, run(miniature(), `<other/>`), "the root element is <other>")
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
	assert.ErrorContains(t, run(miniature(), `<t><e word="a"></t>`), "t.xml")
	assert.ErrorContains(t, run(miniature(), `<t><e word="a" word="b"/></t>`), "twice")
}

func TestEveryProblemIsReportedAtOnce(t *testing.T) {
	err := run(miniature(), `<t><bogus/><e word=""/><e word="a" color="x"/></t>`)
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
	spec := xmlcheck.Spec{
		Root: "t",
		Elems: map[string]xmlcheck.Elem{
			"t": {Children: []string{"r"}},
			"r": {Required: []string{"name"}, Keys: []xmlcheck.Key{{Attrs: []string{"name"}, Split: true}}},
		},
	}
	assert.ErrorContains(t, run(spec, `<t><r name="ls"/><r name="dir, ls"/></t>`), "repeats the one")
	require.NoError(t, run(spec, `<t><r name="ls"/><r name="dir"/></t>`))
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
	root, err := xmlcheck.Parse("x.xml", []byte("<?xml version=\"1.1\"?>\n<t>\n  <e word=\"a\"/>\n</t>\n"))
	require.NoError(t, err)
	require.Len(t, root.Children, 1)
	assert.Equal(t, 3, root.Children[0].Line)
}
