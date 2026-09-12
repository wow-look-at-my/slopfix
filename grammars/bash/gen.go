// Package bash holds the Bash parse table, translated from the grammar
// submodule beside it. The heredoc scanner it needs is hand written, and comes
// from go-tree-sitter rather than being copied here.
package bash

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package bash -standalone -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/bash -out parser.go testdata/tree-sitter-bash/src/parser.c
