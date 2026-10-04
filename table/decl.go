// decl.go lets a loader read the XML version this repository writes.
package table

import "regexp"

// xmlDecl matches the version in an XML declaration, which opens the document.
var xmlDecl = regexp.MustCompile(`^(\s*<\?xml\s+version=")1\.1(")`)

// Readable answers raw with a declaration encoding/xml accepts.
func Readable(raw []byte) []byte {
	return xmlDecl.ReplaceAll(raw, []byte("${1}1.0${2}"))
}
