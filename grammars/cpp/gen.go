// Package cpp holds the C++ parse table, translated from the grammar submodule
// beside it. The raw string scanner it needs is hand written, and comes from
// go-tree-sitter rather than being copied here.
package cpp

import (
	"sync"

	ts "github.com/wow-look-at-my/go-tree-sitter"
)

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package cpp -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/cpp -out parser.gen.go testdata/tree-sitter-cpp/src/parser.c

// load is set by the parser.gen.go that the generate step writes.
var (
	load      func() *ts.Language
	loadOnce  sync.Once
	generated *ts.Language
)

// Language returns the C++ grammar, decoding its tables when a parse needs
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
