// Package clang holds the C parse table, translated from the submodule beside it.
package clang

// A module zip carries the gitlink, not the submodule files, so fetch earliest.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-c -rev b780e47fc780ddc8da13afa35a3f4ed5c157823d -dir testdata/tree-sitter-c
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package clang -out parser.gen.go testdata/tree-sitter-c/src/parser.c

import (
	ts "github.com/wow-look-at-my/go-tree-sitter"

	"github.com/wow-look-at-my/slopfix/grammars/lazy"
)

// load is set by the parser.gen.go the generate step writes, so this package compiles without the tables.
var load func() *ts.Language

var grammar = lazy.New(&load)

// Language returns the grammar, or nil when the generate step has not run.
func Language() *ts.Language { return grammar.Language() }

// Ready reports whether the generate step has run for this grammar.
func Ready() bool { return grammar.Ready() }
