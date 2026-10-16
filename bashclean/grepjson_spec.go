package bashclean

import "github.com/wow-look-at-my/slopfix/xmlcheck"

// GrepJSONSpec is what grepjson.xml is held to. It names programs, flags and
// patterns that a shell reads as typed, so the root says case="sensitive" and
// no entry can say otherwise.
func GrepJSONSpec() xmlcheck.Spec {
	syntaxes := []string{"basic", "extended", "fixed", "perl"}
	elems := map[string]xmlcheck.Elem{
		"grepJSON": {Case: true, Children: []string{"test", "fixture", "flags", "program", "translate"}},
		"test":     {Required: []string{"cmd"}, Attrs: []string{"keep"}, Text: true, Enum: map[string][]string{"keep": {"true", "false"}}},
		"fixture": {
			Required: []string{"name", "kind"}, Text: true,
			Enum: map[string][]string{"kind": {"none", "document", "lines", "one-line"}},
			Keys: []xmlcheck.Key{{Attrs: []string{"name"}}},
		},
		"flags": {Required: []string{"name"}, Children: []string{"flag"}, Keys: []xmlcheck.Key{{Attrs: []string{"name"}}}},
		"flag": {
			Required: []string{"effect"}, Attrs: []string{"short", "long", "value"}, Any: []string{"short", "long"},
			Children: []string{"test"},
			Enum:     map[string][]string{"effect": grepEffectNames, "value": {"optional"}},
			Keys:     []xmlcheck.Key{{Attrs: []string{"short"}}, {Attrs: []string{"long"}}},
		},
		"program": {
			Required: []string{"name", "flags", "syntax"}, Children: []string{"test"},
			Enum: map[string][]string{"syntax": syntaxes},
			Keys: []xmlcheck.Key{{Attrs: []string{"name"}}},
		},
		"translate": {
			Required: []string{"syntax", "from"}, Attrs: []string{"to"},
			Enum: map[string][]string{"syntax": syntaxes},
			Keys: []xmlcheck.Key{{Attrs: []string{"syntax", "from"}}},
		},
	}
	return xmlcheck.Spec{
		Root:      "grepJSON",
		Elems:     elems,
		Test:      xmlcheck.TestDef{Elem: "test", In: "cmd", Out: []string{"keep"}, Body: true, EmptyOut: true},
		Sensitive: true,
	}
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
