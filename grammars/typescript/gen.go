// Package typescript holds the TypeScript parse table, translated from the
// grammar submodule beside it. Nothing here is committed but this file: the
// table is generated at build time and compiled into the binary.
//
// The submodule carries two grammars, typescript and tsx. This takes the first:
// tsx differs only in how it reads angle brackets, which no comment depends on.
package typescript

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package typescript -standalone -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/typescript -out parser.go tree-sitter-typescript/typescript/src/parser.c
