// Package typescript holds the TypeScript parse table, from the submodule beside it.
package typescript

// A module zip carries the gitlink, not the submodule files, so fetch earliest.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-typescript -rev 75b3874edb2dc714fb1fd77a32013d0f8699989f -dir testdata/tree-sitter-typescript
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package typescript -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/typescript -out parser.gen.go testdata/tree-sitter-typescript/typescript/src/parser.c
