package bashclean

import (
	_ "embed"
	"strings"

	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

//go:embed grepjson.xsd
var grepJSONSchema []byte

// GrepJSONSpec is what grepjson.xml is held to. grepjson.xsd owns the shape of
// the file. It pins its root to case="sensitive", because the table names
// programs, flags and patterns that a shell reads as typed.
func GrepJSONSpec() xmlcheck.Spec {
	elems := map[string]xmlcheck.Elem{
		"grepJSON":  {Case: true},
		"fixture":   {Keys: []xmlcheck.Key{{Attrs: []string{"name"}}}},
		"flags":     {Keys: []xmlcheck.Key{{Attrs: []string{"name"}}}},
		"flag":      {Keys: []xmlcheck.Key{{Attrs: []string{"short"}}, {Attrs: []string{"long"}}}},
		"program":   {Keys: []xmlcheck.Key{{Attrs: []string{"name"}}}},
		"translate": {Keys: []xmlcheck.Key{{Attrs: []string{"syntax", "from"}}}},
	}
	return xmlcheck.Spec{
		Schema: grepJSONSchema,
		Elems:  elems,
		Test:   xmlcheck.TestDef{Elem: "test", In: "cmd", Out: []string{"keep"}, Body: true, EmptyOut: true},
		Entry:  checkGrepFlag,
	}
}

// checkGrepFlag answers what a schema cannot say: a flag has a short spelling,
// a long spelling, or both.
func checkGrepFlag(n *xmlcheck.Node, _ bool) []string {
	if n.Name != "flag" {
		return nil
	}
	if strings.TrimSpace(n.Attrs["short"]) != "" || strings.TrimSpace(n.Attrs["long"]) != "" {
		return nil
	}
	return []string{"a <flag> needs one of short, long"}
}

// ValidateGrepJSON holds a grepjson.xml to GrepJSONSpec.
func ValidateGrepJSON(raw []byte) error {
	return xmlcheck.Run(GrepJSONSpec(), xmlcheck.File{Name: "bashclean/grepjson.xml", Raw: raw})
}

// init stops the binary at start-up when the embedded table is invalid.
func init() {
	if err := ValidateGrepJSON(grepJSONXML); err != nil {
		panic("bashclean: " + err.Error())
	}
}
