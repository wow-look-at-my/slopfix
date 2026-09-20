// Package rules carries the folder beside this file into the binary.
//
// The XML is the table a reader edits, and table.Load turns it into the entries
// a consumer runs on. Embedding it is what lets a shipped binary carry the
// tables without a folder beside it.
package rules

import "embed"

//go:embed *.xml
var FS embed.FS
