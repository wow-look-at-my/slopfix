// Package tsx holds the TSX parse table, translated from the TypeScript
// grammar submodule, which ships both grammars. The scanner it needs is hand
// written, and comes from go-tree-sitter rather than being copied here.
package tsx

// tsx reads the typescript grammar's submodule, so it fetches into that
// directory rather than one of its own. ts-fetch is a no-op once the sources
// are there, so whichever package generates first pays for it.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-typescript -rev 75b3874edb2dc714fb1fd77a32013d0f8699989f -dir ../typescript/testdata/tree-sitter-typescript
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package tsx -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/typescript -out parser.gen.go ../typescript/testdata/tree-sitter-typescript/tsx/src/parser.c
