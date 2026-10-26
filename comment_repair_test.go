package slopfix_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// commentRepairTable holds the whole-file cases of the comment repairs.
var commentRepairTable = table.MustLoad(rules.FS, "comment-repair")

// commentRepairFiles names each commentfix file that a comment repair reaches.
var commentRepairFiles = []string{}

// Each case in rules/comment-repair.xml is the file before and after slopfix fix.
func TestEveryCommentRepairTestHolds(t *testing.T) {
	require.NotEmpty(t, commentRepairTable.Tests)

	for _, c := range commentRepairTable.Tests {
		t.Run(c.Path, func(t *testing.T) {
			require.NotEmpty(t, c.Path, "a whole-file case names its file")
			req := slopfix.Request{Content: c.In, Path: c.Path, MaxCommentLines: tombstones.DefaultMaxCommentLines}
			repair := slopfix.Fix(req)
			assert.Equal(t, c.Out, repair.Text)

			req.Content = repair.Text
			again := slopfix.Fix(req)
			assert.Equal(t, repair.Text, again.Text, "a second fix changes nothing")
			for _, f := range again.Findings {
				assert.True(t, f.Warning(), "only a warning stays: %s line %d %s", f.ID, f.Line, f.Detail)
			}
		})
	}
}

// The guard parses commentfix. It starts at each Repair of a fixer.Spec and at
// CapLines, and collects the file of each function it reaches. That set must
// equal commentRepairFiles. It checks the names, not that a case drives each file.
func TestEveryCommentRepairFileIsNamed(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("commentfix", "*.go"))
	require.NoError(t, err)

	fset := token.NewFileSet()
	decls := map[string][]*ast.FuncDecl{}
	declFile := map[*ast.FuncDecl]string{}
	reached := map[string]bool{}
	var queue []ast.Node
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		name := filepath.Base(path)
		for _, d := range file.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				decls[fn.Name.Name] = append(decls[fn.Name.Name], fn)
				declFile[fn] = name
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Repair" {
				if _, ok := kv.Value.(*ast.FuncLit); ok {
					reached[name] = true
				}
				queue = append(queue, kv.Value)
			}
			return true
		})
	}
	queue = append(queue, ast.NewIdent("CapLines"))

	seen := map[*ast.FuncDecl]bool{}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		ast.Inspect(node, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for _, fn := range decls[id.Name] {
				if seen[fn] {
					continue
				}
				seen[fn] = true
				reached[declFile[fn]] = true
				if fn.Body != nil {
					queue = append(queue, fn.Body)
				}
			}
			return true
		})
	}

	var got []string
	for name := range reached {
		got = append(got, name)
	}
	slices.Sort(got)
	want := slices.Clone(commentRepairFiles)
	slices.Sort(want)
	assert.Equal(t, want, got, "commentRepairFiles must name each commentfix file a repair reaches")
}
