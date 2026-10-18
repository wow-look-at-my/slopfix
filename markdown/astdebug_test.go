package markdown

import (
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func TestDebugAST(t *testing.T) {
	for _, doc := range []string{
		"<div>\n  <defs><style>text { font-family: Arial, sans-serif; }</style></defs>\n</div>\n",
		"<details>\n<summary>More</summary>\n</details>\n",
		"<div>\n<defs>x</defs>\n</div>\n",
	} {
		src := []byte(doc)
		t.Logf("DOC %q", doc)
		_ = ast.Walk(parser.Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering {
				t.Logf("  node %s lines=%d", n.Kind(), n.Lines().Len())
			}
			return ast.WalkContinue, nil
		})
	}
}
