// Package javascript holds the JavaScript parse table, from the submodule beside it.
package javascript

// A module zip carries the gitlink, not the submodule files, so fetch earliest.
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-fetch -repo tree-sitter/tree-sitter-javascript -rev 58404d8cf191d69f2674a8fd507bd5776f46cb11 -dir testdata/tree-sitter-javascript
//go:generate go run github.com/wow-look-at-my/go-tree-sitter/cmd/ts-translate -package javascript -scanner github.com/wow-look-at-my/go-tree-sitter/grammars/javascript -out parser.gen.go testdata/tree-sitter-javascript/src/parser.c
