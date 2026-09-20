// decode.go turns a rules document into the structs above. The reader parses
// XML 1.1, which the standard library refuses outright, and this walks what it
// answers rather than binding by reflection: every attribute a rule carries is
// named here, so a typo in a rules file is a missing value at a named line
// rather than a field that silently stayed empty.
package gen

import (
	"fmt"
	"strings"

	"github.com/wow-look-at-my/xml-validator/reader"
)

// decodeFile reads one rules document.
func decodeFile(raw []byte) (file, error) {
	doc, err := reader.ParseTree(strings.NewReader(string(raw)))
	if err != nil {
		return file{}, err
	}
	if doc == nil || doc.Root == nil {
		return file{}, fmt.Errorf("no root element")
	}
	out := file{For: attr(doc.Root, "for")}
	for _, el := range doc.Root.ChildElements() {
		switch el.Name.Local {
		case "drop":
			out.Drops = append(out.Drops, Drop{
				ID: attr(el, "id"), Word: attr(el, "word"),
				Where: attr(el, "where"), Tests: decodeTests(el),
			})
		case "rewrite":
			out.Rewrites = append(out.Rewrites, Rewrite{
				ID: attr(el, "id"), From: attr(el, "from"), To: attr(el, "to"),
				Where: attr(el, "where"), Tests: decodeTests(el),
			})
		case "pattern":
			out.Patterns = append(out.Patterns, Pattern{
				ID: attr(el, "id"), Match: attr(el, "match"), To: attr(el, "to"),
				Where: attr(el, "where"), Tests: decodeTests(el),
			})
		case "flag":
			out.Flags = append(out.Flags, Flag{
				ID: attr(el, "id"), Phrase: attr(el, "phrase"),
				Say: attr(el, "say"), Tests: decodeTests(el),
			})
		case "class":
			out.Classes = append(out.Classes, Class{
				Name: attr(el, "name"), Words: attr(el, "words"),
				Suffix: attr(el, "suffix"), Open: attr(el, "open") == "true",
				Except: attr(el, "except"),
			})
		case "rephrase":
			out.Rephrases = append(out.Rephrases, Rephrase{
				ID: attr(el, "id"), Match: attr(el, "match"), To: attr(el, "to"),
				Where: attr(el, "where"), Tests: decodeTests(el),
			})
		case "normalize":
			out.Normals = append(out.Normals, Normalize{
				ID: attr(el, "id"), From: attr(el, "from"), To: attr(el, "to"),
				Tests: decodeTests(el),
			})
		default:
			return file{}, fmt.Errorf("<%s> is not a rule element", el.Name.Local)
		}
	}
	return out, nil
}

// decodeTests reads the tests nested in a rule element.
func decodeTests(el *reader.Element) []Test {
	var out []Test
	for _, child := range el.ChildElements() {
		if child.Name.Local != "test" {
			continue
		}
		out = append(out, Test{In: attr(child, "in"), Out: attr(child, "out")})
	}
	return out
}

// attr answers an attribute, empty when the element does not carry it.
func attr(el *reader.Element, name string) string {
	value, _ := el.Attr(name)
	return value
}
