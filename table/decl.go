// decl.go lets a loader read the XML version this repository writes.
package table

import "github.com/wow-look-at-my/slopfix/xmlcheck"

// Readable answers raw with a declaration encoding/xml accepts.
func Readable(raw []byte) []byte { return xmlcheck.Readable(raw) }
