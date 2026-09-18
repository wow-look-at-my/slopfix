// Package tsx holds the TSX parse table, from the TypeScript submodule.
package tsx

// tsx fetches into the typescript package's directory, and ts-fetch no-ops a
// single time the sources are there.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-typescript -rev 75b3874edb2dc714fb1fd77a32013d0f8699989f -dir ../typescript/testdata/tree-sitter-typescript
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package tsx -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/typescript -out parser.gen.go ../typescript/testdata/tree-sitter-typescript/tsx/src/parser.c
