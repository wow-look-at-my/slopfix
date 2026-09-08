// Package tsx holds the TSX parse table, translated from the TypeScript
// grammar submodule, which ships both grammars. The scanner it needs is hand
// written, and comes from go-tree-sitter rather than being copied here.
package tsx

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package tsx -standalone -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/typescript -out parser.go ../typescript/tree-sitter-typescript/tsx/src/parser.c
