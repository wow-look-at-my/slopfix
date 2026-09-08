// Package cpp holds the C++ parse table, translated from the grammar submodule
// beside it. The raw string scanner it needs is hand written, and comes from
// go-tree-sitter rather than being copied here.
package cpp

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package cpp -standalone -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/cpp -out parser.go testdata/tree-sitter-cpp/src/parser.c
