// Package typescript holds the TypeScript parse table, translated from the
// grammar submodule beside it. The scanner it needs is hand written, and comes
// from go-tree-sitter rather than being copied here.
package typescript

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package typescript -standalone -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/typescript -out parser.go testdata/tree-sitter-typescript/typescript/src/parser.c
