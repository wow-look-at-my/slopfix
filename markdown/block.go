// Package markdown finds prose with a CommonMark parser.
// Fences, tables, headings and quotes stay verbatim.
package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/wow-look-at-my/slopfix/trace"
)

// Kind names what a block is, which decides whether prose rules reach it.
type Kind int

const (
	// Prose is a paragraph or a list item. It is rewritten and checked.
	Prose Kind = iota
	// Verbatim is anything else, such as a fence, a heading or a quotation.
	Verbatim
)

// Block is a run of lines, and what may be done to it.
type Block struct {
	Kind Kind
	// Lines are the source lines, in order, without their line endings.
	Lines []string
	// Start is where Lines begins in the source, counting from the top.
	Start int
	// Marker is the list bullet the item opened with, empty for a paragraph.
	Marker string
	// Indent is the whitespace before the block.
	Indent string
}

// Text renders a prose block as the single line the rules read: the lines
// joined by a space, with each continuation's own indentation dropped.
func (b Block) Text() string {
	parts := make([]string, 0, len(b.Lines))
	for i, line := range b.Lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 {
			trimmed = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), b.Marker))
		}
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, " ")
}

// Split walks the document and returns its blocks in order.
func Split(content string) []Block {
	defer trace.Phase("markdown/split")()
	lines := strings.Split(content, "\n")
	ends := paragraphs(content, lines)
	var blocks []Block
	for i := 0; i < len(lines); {
		end, prose := ends[i]
		if !prose {
			blocks = append(blocks, Block{Kind: Verbatim, Lines: lines[i : i+1], Start: i + 1})
			i++
			continue
		}
		indent, marker := listMarker(lines[i])
		if marker == "" {
			indent = lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
		}
		blocks = append(blocks, Block{Kind: Prose, Lines: lines[i : end+1], Start: i + 1, Marker: marker, Indent: indent})
		i = end + 1
	}
	return blocks
}

var parser = goldmark.New(goldmark.WithExtensions(extension.Table)).Parser()

// paragraphs maps the line each paragraph opens on to the line it ends on. A
// paragraph in a block quote quotes somebody, and stays as they wrote it.
func paragraphs(content string, lines []string) map[int]int {
	src := []byte(blankFrontMatter(content, lines))
	starts := lineStarts(lines)
	ends := map[int]int{}
	_ = ast.Walk(parser.Parse(text.NewReader(src)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.Kind() {
		case ast.KindBlockquote:
			return ast.WalkSkipChildren, nil
		case ast.KindParagraph, ast.KindTextBlock:
			if segments := n.Lines(); segments.Len() > 0 {
				first := lineOf(starts, segments.At(0).Start)
				ends[first] = lineOf(starts, segments.At(segments.Len()-1).Stop-1)
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return ends
}

// blankFrontMatter writes spaces over a YAML front matter block, byte for byte,
// so the parser reads no paragraph there and every offset holds.
func blankFrontMatter(content string, lines []string) string {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content
	}
	end := len(lines[0]) + 1
	for j := 1; j < len(lines); j++ {
		end += len(lines[j]) + 1
		if strings.TrimSpace(lines[j]) != "---" {
			continue
		}
		out := []byte(content)
		for k := range min(end, len(out)) {
			if out[k] != '\n' {
				out[k] = ' '
			}
		}
		return string(out)
	}
	return content
}

// lineStarts answers the byte offset each line starts at.
func lineStarts(lines []string) []int {
	starts := make([]int, len(lines))
	at := 0
	for i, line := range lines {
		starts[i] = at
		at += len(line) + 1
	}
	return starts
}

// lineOf answers the line that holds the byte at offset at.
func lineOf(starts []int, at int) int {
	lo, hi := 0, len(starts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if starts[mid] <= at {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// listMarker splits a list line into its leading whitespace and its bullet or
// number. Both are empty when the line opens no list item.
func listMarker(line string) (indent, marker string) {
	trimmed := strings.TrimLeft(line, " \t")
	indent = line[:len(line)-len(trimmed)]
	for _, bullet := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trimmed, bullet) {
			return indent, strings.TrimSpace(bullet)
		}
	}
	// An ordered item: digits, then a dot or a paren, then a space.
	for n := 0; n < len(trimmed); n++ {
		if trimmed[n] >= '0' && trimmed[n] <= '9' {
			continue
		}
		if n > 0 && (trimmed[n] == '.' || trimmed[n] == ')') && n+1 < len(trimmed) && trimmed[n+1] == ' ' {
			return indent, trimmed[:n+1]
		}
		break
	}
	return "", ""
}
