// Package bash holds the Bash parse table, translated from the submodule beside it.
package bash

// A module zip carries the gitlink, not the submodule files, so fetch earliest.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-bash -rev a06c2e4415e9bc0346c6b86d401879ffb44058f7 -dir testdata/tree-sitter-bash
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package bash -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/bash -out parser.gen.go testdata/tree-sitter-bash/src/parser.c
