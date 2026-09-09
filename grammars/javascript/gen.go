// Package javascript holds the JavaScript parse table, translated from the
// grammar submodule beside it. The scanner it needs is hand written, and comes
// from go-tree-sitter rather than being copied here.
package javascript

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package javascript -standalone -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/javascript -out parser.go testdata/tree-sitter-javascript/src/parser.c
