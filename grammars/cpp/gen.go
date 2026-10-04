// Package cpp holds the C++ parse table, translated from the submodule beside it.
package cpp

// A module zip carries the gitlink, not the submodule files, so fetch earliest.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-cpp -rev 8b5b49eb196bec7040441bee33b2c9a4838d6967 -dir testdata/tree-sitter-cpp
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package cpp -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/cpp -out parser.gen.go testdata/tree-sitter-cpp/src/parser.c

import (
	ts "github.com/wow-look-at-my/go-tree-sitter"

	"github.com/wow-look-at-my/slopfix/grammars/lazy"
)

// load is set by the parser.gen.go the generate step writes, so this package compiles without the tables.
var load func() *ts.Language

var loader = lazy.New(&load)

// Language returns the grammar, or nil when the generate step has not run.
func Language() *ts.Language { return loader.Language() }

// Ready reports whether the generate step has run for this grammar.
func Ready() bool { return loader.Ready() }
