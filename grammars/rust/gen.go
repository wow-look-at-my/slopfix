// Package rust holds the Rust parse table, translated from the grammar
// submodule beside it. The scanner it needs is hand written, and comes from
// go-tree-sitter rather than being copied here.
package rust

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package rust -standalone -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/rust -out parser.gen.go testdata/tree-sitter-rust/src/parser.c
