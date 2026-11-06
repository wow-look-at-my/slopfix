// Spec is what rules.xml is held to.

package autoallow

import (
	_ "embed"
	"slices"
	"strings"

	"github.com/wow-look-at-my/slopfix/xmlcheck"
)

//go:embed rules.xsd
var schema []byte

// Spec is what rules.xml is held to. rules.xsd owns the shape of the file and
// pins its root to case="sensitive". This is because the table names shell
// words, flags and paths, which a shell reads as typed.
func Spec() xmlcheck.Spec {
	elems := map[string]xmlcheck.Elem{
		"rules":     {Case: true},
		"denyPath":  {Keys: []xmlcheck.Key{{Attrs: []string{"prefix"}}}},
		"mcpServer": {Keys: []xmlcheck.Key{{Attrs: []string{"name"}}}},
		"tool":      {Keys: []xmlcheck.Key{{Attrs: []string{"#text"}}}},
		"rule":      {Keys: []xmlcheck.Key{{Attrs: []string{"name"}, Split: true}, {Attrs: []string{"command"}}}},
		"flag":      {Keys: []xmlcheck.Key{{Attrs: []string{"name"}}}},
		"value":     {Keys: []xmlcheck.Key{{Attrs: []string{"#text"}}}},
	}
	return xmlcheck.Spec{
		Schema: schema,
		Elems:  elems,
		Test:   xmlcheck.TestDef{Elem: "test", In: "cmd", Out: []string{"expect"}, EmptyOut: true},
		Entry:  checkRule,
	}
}

// checkRule answers what a schema cannot say: a rule matches by name or by
// command, and needs one of those.
func checkRule(n *xmlcheck.Node, _ bool) []string {
	if n.Name != "rule" {
		return nil
	}
	names := []string{n.Attrs["name"], n.Attrs["command"]}
	if slices.ContainsFunc(names, func(s string) bool { return strings.TrimSpace(s) != "" }) {
		return nil
	}
	return []string{"a <rule> needs one of name, command"}
}

// Validate holds a rules file to Spec.
func Validate(data []byte) error {
	return xmlcheck.Run(Spec(), xmlcheck.File{Name: "autoallow/rules.xml", Raw: data})
}

// init stops the binary at start-up when the embedded table is invalid, rather
// than letting a rule that cannot fire sit in it.
func init() {
	if err := Validate(rulesXML); err != nil {
		panic("autoallow: " + err.Error())
	}
}
