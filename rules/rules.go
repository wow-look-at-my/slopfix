// Package rules carries the prose tables into the binary. The XML beside this
// file is the table a reader edits, and it is embedded rather than read from
// disk so a shipped binary needs no folder next to it.
package rules

import "embed"

//go:embed *.xml
var FS embed.FS
