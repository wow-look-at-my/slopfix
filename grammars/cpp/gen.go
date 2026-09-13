// Package cpp holds the C++ parse table, translated from the grammar submodule
// beside it. The raw string scanner it needs is hand written, and comes from
// go-tree-sitter rather than being copied here.
package cpp

// A module zip carries the gitlink and none of the submodule's files, so a
// consumer has to fetch the sources before anything can translate them.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-cpp -rev 8b5b49eb196bec7040441bee33b2c9a4838d6967 -dir testdata/tree-sitter-cpp
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package cpp -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/cpp -out parser.gen.go testdata/tree-sitter-cpp/src/parser.c
