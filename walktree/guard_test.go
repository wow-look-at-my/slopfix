package walktree

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// dirReads are the calls that read a directory. Outside this package every
// one of them is a second walk waiting to happen.
var dirReads = map[string]bool{
	"filepath.Walk":    true,
	"filepath.WalkDir": true,
	"fs.WalkDir":       true,
	"os.ReadDir":       true,
	"fs.ReadDir":       true,
}

// dirMethods are the *os.File methods that read a directory.
var dirMethods = map[string]bool{"ReadDir": true, "Readdir": true, "Readdirnames": true}

// TestOnlyWalktreeReadsDirectories fails the build on any non-test Go file
// outside this package that reads a directory itself.
func TestOnlyWalktreeReadsDirectories(t *testing.T) {
	module, err := filepath.Abs("..")
	require.NoError(t, err)
	enter := func(dir string) bool {
		name := filepath.Base(dir)
		return !strings.HasPrefix(name, ".") && name != "testdata" && name != "grammars" && name != "spec" && name != "build" && name != "walktree"
	}
	var sources []string
	Get(module).Visit(enter, func(path string, e Entry) {
		if e.IsRegular() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			sources = append(sources, path)
		}
	})
	require.NotEmpty(t, sources)

	var offenders []string
	fset := token.NewFileSet()
	for _, path := range sources {
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			name := sel.Sel.Name
			if pkg, ok := sel.X.(*ast.Ident); ok && dirReads[pkg.Name+"."+name] {
				offenders = append(offenders, fset.Position(call.Pos()).String()+": "+pkg.Name+"."+name)
				return true
			}
			if _, isPkg := sel.X.(*ast.Ident); dirMethods[name] && !(isPkg && sel.X.(*ast.Ident).Name == "walktree") {
				offenders = append(offenders, fset.Position(call.Pos()).String()+": ."+name)
			}
			return true
		})
	}
	require.Empty(t, offenders, "read directories through walktree.Get or walktree.List")
}
