// comments.go exposes every comment a file carries, for a rule that reads the
// prose rather than its size.
//
// It sits beside the parse because the grammar map and the "a comment is a node
// whose type carries comment" fact already live there. Another extractor is
// another answer to which files have comments.
package code

import ts "github.com/wow-look-at-my/go-tree-sitter"

// Comment is a comment node.
type Comment struct {
	// Text is the comment as written, markers included.
	Text string
	// Offset is where Text starts in the file.
	Offset int
}

// Comments returns every comment in a file, in file order.
func Comments(filename, src string) []Comment {
	language := LanguageFor(filename)
	if language == nil {
		return nil
	}
	tree := Tree(language, src)
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

// collectComments walks every named node and keeps the comments.
func collectComments(node ts.Node, src string, out *[]Comment) {
	count := node.NamedChildCount()
	for i := uint32(0); i < count; i++ {
		child := node.NamedChild(i)
		if IsComment(child) {
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
