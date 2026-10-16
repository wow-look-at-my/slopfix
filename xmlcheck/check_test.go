package xmlcheck_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

// miniatureSchema is the shape of the small table: a root holding entries
// (e), entries that take no case (f), named rows (r) and tests.
const miniatureSchema = `<?xml version="1.1" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified">
  <xs:simpleType name="text">
    <xs:restriction base="xs:string">
      <xs:pattern value="[\s\S]*\S[\s\S]*"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="case">
    <xs:restriction base="xs:string">
      <xs:enumeration value="sensitive"/>
      <xs:enumeration value="insensitive"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="level">
    <xs:restriction base="xs:string">
      <xs:enumeration value="low"/>
      <xs:enumeration value="high"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="surface">
    <xs:restriction base="xs:string">
      <xs:enumeration value="a"/>
      <xs:enumeration value="b"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="surfaces">
    <xs:list itemType="surface"/>
  </xs:simpleType>
  <xs:complexType name="test">
    <xs:attribute name="in" type="text" use="required"/>
    <xs:attribute name="out" type="xs:string"/>
  </xs:complexType>
  <xs:element name="t">
    <xs:complexType>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
        <xs:element name="test" type="test"/>
        <xs:element name="e">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="test" type="test" minOccurs="0" maxOccurs="unbounded"/>
            </xs:sequence>
            <xs:attribute name="word" type="text" use="required"/>
            <xs:attribute name="level" type="level"/>
            <xs:attribute name="where" type="surfaces"/>
            <xs:attribute name="probe" type="xs:string"/>
            <xs:attribute name="expect" type="xs:string"/>
            <xs:attribute name="case" type="case"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="f">
          <xs:complexType>
            <xs:attribute name="word" type="text" use="required"/>
            <xs:attribute name="case" type="case"/>
          </xs:complexType>
        </xs:element>
        <xs:element name="r">
          <xs:complexType>
            <xs:attribute name="name" type="text" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:choice>
      <xs:attribute name="case" type="case"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

// miniature is a small table that exercises each kind of check.
func miniature() xmlcheck.Spec {
	return xmlcheck.Spec{
		Schema: []byte(miniatureSchema),
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
	root, err := xmlcheck.Parse("x.xml", []byte("<?xml version=\"1.1\"?>\n<t>\n  <e word=\"a\"/>\n</t>\n"))
	require.NoError(t, err)
	require.Len(t, root.Children, 1)
	assert.Equal(t, 3, root.Children[0].Line)
}
