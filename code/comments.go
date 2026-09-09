// comments.go exposes every comment a file carries, for a rule that reads the
// prose rather than its size.
//
// It is here and not in a package of its own because the parse, the grammar map
// and the "a comment is a node whose type carries comment" fact already live
// here. Another extractor is another answer to which files have comments.
package commentlength

import ts "github.com/wow-look-at-my/go-tree-sitter"

// Comment is a comment node, with the byte offset a caller counts from.
type Comment struct {
	// Text is the comment as written, markers included.
	Text string
	// Offset is where Text starts in the file.
	Offset int
}

// Comments returns every comment in a file, in file order.
//
// A file carrying a syntax error still answers, unlike the block pairing beside
// it. That pairing needs the construct under a comment, and a wrong span there
// deletes the wrong prose. A comment node carries its own span. A hook reads a
// file mid-edit routinely, and a rule quiet exactly then enforces nothing.
func Comments(filename, src string) []Comment {
	language := languageFor(filename)
	if language == nil {
		return nil
	}
	parser := ts.NewParser()
	if !parser.SetLanguage(language) {
		return nil
	}
	tree := parser.ParseString(nil, []byte(src))
	if tree == nil {
		return nil
	}
	root := tree.RootNode()
	if root.IsNull() {
		return nil
	}
	var out []Comment
	collectComments(root, src, &out)
	return out
}

// collectComments walks every named node and keeps the comments. A comment
// inside a function body is found the way a comment above a declaration is.
func collectComments(node ts.Node, src string, out *[]Comment) {
	count := node.NamedChildCount()
	for i := uint32(0); i < count; i++ {
		child := node.NamedChild(i)
		if isComment(child) {
			start, end := int(child.StartByte()), int(child.EndByte())
			if start < 0 || end > len(src) || start >= end {
				continue
			}
			*out = append(*out, Comment{Text: src[start:end], Offset: start})
			continue
		}
		collectComments(child, src, out)
	}
}
