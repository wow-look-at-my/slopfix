// Package javascript holds the JavaScript parse table, translated from the
// grammar submodule beside it. The scanner it needs is hand written, and comes
// from go-tree-sitter rather than being copied here.
package javascript

// A module zip carries the gitlink and none of the submodule's files, so a
// consumer has to fetch the sources before anything can translate them.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-javascript -rev 58404d8cf191d69f2674a8fd507bd5776f46cb11 -dir testdata/tree-sitter-javascript
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package javascript -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/javascript -out parser.gen.go testdata/tree-sitter-javascript/src/parser.c
