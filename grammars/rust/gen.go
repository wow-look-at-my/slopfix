// Package rust holds the Rust parse table, translated from the grammar
// submodule beside it. The scanner it needs is hand written, and comes from
// go-tree-sitter rather than being copied here.
package rust

// A module zip carries the gitlink and none of the submodule's files, so a
// consumer has to fetch the sources before anything can translate them.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-rust -rev 77a3747266f4d621d0757825e6b11edcbf991ca5 -dir testdata/tree-sitter-rust
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package rust -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/rust -out parser.gen.go testdata/tree-sitter-rust/src/parser.c
