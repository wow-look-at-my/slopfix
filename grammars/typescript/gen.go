// Package typescript holds the TypeScript parse table, translated from the
// grammar submodule beside it. The scanner it needs is hand written, and comes
// from go-tree-sitter rather than being copied here.
package typescript

// A module zip carries the gitlink and none of the submodule's files, so a
// consumer has to fetch the sources before anything can translate them.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-typescript -rev 75b3874edb2dc714fb1fd77a32013d0f8699989f -dir testdata/tree-sitter-typescript
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package typescript -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/typescript -out parser.gen.go testdata/tree-sitter-typescript/typescript/src/parser.c
