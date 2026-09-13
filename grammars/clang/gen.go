// Package clang holds the C parse table, translated from the grammar submodule
// beside it. Nothing here is committed but this file: the table is generated at
// build time and compiled into the binary.
package clang

// A module zip carries the gitlink and none of the submodule's files, so a
// consumer has to fetch the sources before anything can translate them.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-c -rev b780e47fc780ddc8da13afa35a3f4ed5c157823d -dir testdata/tree-sitter-c
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package clang -out parser.gen.go testdata/tree-sitter-c/src/parser.c
