// Package golang holds the Go parse table, translated from the grammar
// submodule beside it. Nothing here is committed but this file: the table is
// generated at build time and compiled into the binary.
package golang

// A module zip carries the gitlink and none of the submodule's files, so a
// consumer has to fetch the sources before anything can translate them.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-go -rev 2346a3ab1bb3857b48b29d779a1ef9799a248cd7 -dir testdata/tree-sitter-go
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package golang -out parser.gen.go testdata/tree-sitter-go/src/parser.c
