// Package tsx holds the TSX parse table, from the TypeScript submodule.
package tsx

// tsx fetches into the typescript package's directory, and ts-fetch no-ops a
// single time the sources are there.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-typescript -rev 75b3874edb2dc714fb1fd77a32013d0f8699989f -dir ../typescript/testdata/tree-sitter-typescript
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package tsx -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/typescript -out parser.gen.go ../typescript/testdata/tree-sitter-typescript/tsx/src/parser.c

import (
	ts "github.com/wow-look-at-my/go-tree-sitter"

	"github.com/wow-look-at-my/slopfix/grammars/lazy"
)

// load is set by the parser.gen.go the generate step writes, so this package compiles without the tables.
var load func() *ts.Language

var grammar = lazy.New(&load)

// Language returns the grammar, or nil when the generate step has not run.
func Language() *ts.Language { return grammar.Language() }

// Ready reports whether the generate step has run for this grammar.
func Ready() bool { return grammar.Ready() }
