// Package java holds the Java parse table, translated from the grammar
// submodule beside it. Nothing here is committed but this file: the table is
// generated at build time and compiled into the binary.
package java

//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package java -standalone -out parser.go tree-sitter-java/src/parser.c
