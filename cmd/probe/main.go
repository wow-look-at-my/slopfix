// Command probe dumps the parse of a file, for diagnosis only.
package main

import (
	"fmt"
	"os"

	ts "github.com/wow-look-at-my/go-tree-sitter"
	"github.com/wow-look-at-my/go-tree-sitter/grammars/golang"
)

func main() {
	src, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	p := ts.NewParser()
	p.SetLanguage(golang.Language())
	tree := p.ParseString(nil, src)
	root := tree.RootNode()
	fmt.Println("hasError", root.HasError(), "children", root.NamedChildCount())
	walk(root, 0)
}

func walk(n ts.Node, depth int) {
	fmt.Printf("%*s%s %d-%d\n", depth*2, "", n.Type(), n.StartPoint().Row+1, n.EndPoint().Row+1)
	if depth > 3 {
		return
	}
	for i := uint32(0); i < n.NamedChildCount(); i++ {
		walk(n.NamedChild(i), depth+1)
	}
}
