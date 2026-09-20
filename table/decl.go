// decl.go lets a loader read the XML version this repository writes.
//
// Every document here declares version 1.1. encoding/xml refuses that outright
// with "unsupported version", and it is the only part of 1.1 it cannot take:
// the two versions differ in which characters a name and a character reference
// may carry, and these documents are ASCII. So the declaration is presented as
// 1.0 to the parser and the file on disk keeps the version it states.
package table

import "regexp"

// xmlDecl matches the version in an XML declaration, which opens the document.
var xmlDecl = regexp.MustCompile(`^(\s*<\?xml\s+version=")1\.1(")`)

// Readable answers raw with a declaration encoding/xml accepts.
func Readable(raw []byte) []byte {
	return xmlDecl.ReplaceAll(raw, []byte("${1}1.0${2}"))
}
