// Package javascript holds the JavaScript parse table, translated from the
// grammar submodule beside it. Nothing here is committed but this file: the
// table is generated at build time and compiled into the binary.
package javascript

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package javascript -standalone -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/javascript -out parser.go tree-sitter-javascript/src/parser.c
