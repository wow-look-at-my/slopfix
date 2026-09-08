// Package clang holds the C parse table, translated from the grammar submodule
// beside it. Nothing here is committed but this file: the table is generated at
// build time and compiled into the binary.
package clang

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package clang -standalone -out parser.go tree-sitter-c/src/parser.c
