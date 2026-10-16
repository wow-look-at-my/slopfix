package english

import (
	"github.com/wow-look-at-my/slopfix/table"
	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

// Spec is what english.xml is held to. english.xsd owns the shape of the file.
// Matching ignores case unless the file or an entry says case="sensitive".
func Spec() xmlcheck.Spec {
	inline := func(e xmlcheck.Elem) xmlcheck.Elem {
		e.TestAttr = "test"
		e.Case = true
		return e
	}
	elems := map[string]xmlcheck.Elem{
		"english": {Case: true},
		"class":   {Keys: []xmlcheck.Key{{Attrs: []string{"name"}}}},
		"drop": inline(xmlcheck.Elem{
			Keys: []xmlcheck.Key{{Attrs: []string{"word"}, Exact: []string{"where"}}},
		}),
		"rewrite": inline(xmlcheck.Elem{
			Keys: []xmlcheck.Key{{Attrs: []string{"from"}, Exact: []string{"where"}}},
		}),
		"flag": inline(xmlcheck.Elem{
			Keys: []xmlcheck.Key{{Attrs: []string{"phrase"}, Exact: []string{"where"}}},
		}),
		"pattern": inline(xmlcheck.Elem{
			ExpectAttr: "expect",
			Keys:       []xmlcheck.Key{{Attrs: []string{"match"}, Exact: []string{"where"}}},
		}),
		"shape": inline(xmlcheck.Elem{
			ExpectAttr: "expect",
			Keys:       []xmlcheck.Key{{Attrs: []string{"match"}, Exact: []string{"where"}}},
		}),
	}
	return xmlcheck.Spec{
		Schema:   schema,
		Elems:    elems,
		Test:     xmlcheck.TestDef{Elem: "test", In: "in", Out: []string{"out"}},
		Shadows:  []xmlcheck.Shadow{{Elem: "rewrite", Attr: "from"}, {Elem: "drop", Attr: "word"}},
		Entry:    checkEntry,
		Fires:    entryFires,
		FiresAny: anyEntryFires,
	}
}

// checkEntry answers what makes an entry unusable once its attributes are sound.
func checkEntry(n *xmlcheck.Node, sensitive bool) []string {
	switch n.Name {
	case "pattern":
		return xmlcheck.PatternProblems(n.Attrs["match"], n.Attrs["replace"], sensitive)
	case "rewrite":
		return xmlcheck.RewriteProblems(n.Attrs["from"], n.Attrs["to"], sensitive)
	case "shape":
		return table.CaptureProblems(n.Attrs["match"], n.Attrs["replace"])
	}
	return nil
}

// entryFires answers whether an entry matches the input of one of its tests. A
// shape reads word classes the file is still being loaded to define, and its
// own cases run through Fix.
func entryFires(n *xmlcheck.Node, sensitive bool, in string) bool {
	switch n.Name {
	case "drop":
		return xmlcheck.HasPhrase(in, n.Attrs["word"], sensitive)
	case "rewrite":
		return xmlcheck.HasPhrase(in, n.Attrs["from"], sensitive)
	case "flag":
		return xmlcheck.HasPhrase(in, n.Attrs["phrase"], sensitive)
	case "pattern":
		re, err := xmlcheck.Compile(n.Attrs["match"], sensitive)
		return err != nil || re.MatchString(in)
	}
	return true
}

// anyEntryFires answers whether a drop, a rewrite, a flag or a pattern of the
// file matches an input.
func anyEntryFires(roots []*xmlcheck.Node, in string) bool {
	for _, root := range roots {
		for _, n := range root.Children {
			switch n.Name {
			case "drop", "rewrite", "flag", "pattern":
				if entryFires(n, n.Sensitive(), in) {
					return true
				}
			}
		}
	}
	return false
}
