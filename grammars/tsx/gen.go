// Package tsx holds the TSX parse table, translated from the TypeScript
// grammar submodule, which ships both grammars. The scanner it needs is hand
// written, and comes from go-tree-sitter rather than being copied here.
package tsx

import (
	"sync"

	ts "github.com/wow-look-at-my/go-tree-sitter"
)

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package tsx -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/typescript -out parser.gen.go ../typescript/testdata/tree-sitter-typescript/tsx/src/parser.c

// load is set by the parser.gen.go that the generate step writes.
var (
	load      func() *ts.Language
	loadOnce  sync.Once
	generated *ts.Language
)

// Language returns the TSX grammar, decoding its tables when a parse needs
// them, and nil before the generate step has run.
func Language() *ts.Language {
	loadOnce.Do(func() {
		if load != nil {
			generated = load()
		}
	})
	return generated
}

// Ready reports whether the generate step has run for this grammar.
func Ready() bool {
	loadOnce.Do(func() {
		if load != nil {
			generated = load()
		}
	})
	return generated != nil
}
