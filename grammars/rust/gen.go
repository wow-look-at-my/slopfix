// Package rust holds the Rust parse table, translated from the submodule beside it.
package rust

// A module zip carries the gitlink, not the submodule files, so fetch earliest.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-rust -rev 77a3747266f4d621d0757825e6b11edcbf991ca5 -dir testdata/tree-sitter-rust
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package rust -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/rust -out parser.gen.go testdata/tree-sitter-rust/src/parser.c
