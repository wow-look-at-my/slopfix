// Package markdown splits a document into the blocks the prose rules apply to.
//
// Only prose is rewritten or checked. A fenced code block is data, a table is a
// grid whose rows are not sentences, and a heading is a label. Each of those
// reaches the caller marked as verbatim, so a rule can never reflow one.
package markdown

import "strings"

// Kind names what a block is, which decides whether prose rules reach it.
type Kind int

const (
	// Prose is a paragraph, or one item of a list. It is rewritten and checked.
	Prose Kind = iota
	// Verbatim is a fence, a table, a heading, or a blank run. It is left alone.
	Verbatim
)

// Block is one run of lines, and what may be done to it.
type Block struct {
	Kind Kind
	// Lines are the source lines, in order, without their line endings.
	Lines []string
	// Start is the 1-based line number of Lines[0] in the source.
	Start int
	// Marker is the list bullet or number the item opened with, empty for a
	// paragraph. Joining an item has to put it back.
	Marker string
	// Indent is the whitespace before Marker, preserved for a nested item.
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

// Split walks the document once and returns its blocks in order.
func Split(content string) []Block {
	lines := strings.Split(content, "\n")
	var blocks []Block
	for i := 0; i < len(lines); {
		line := lines[i]
		switch {
		case fenceDelimiter(line) != "":
			end := closingFence(lines, i)
			blocks = append(blocks, Block{Kind: Verbatim, Lines: lines[i : end+1], Start: i + 1})
			i = end + 1
		case isVerbatimLine(line):
			blocks = append(blocks, Block{Kind: Verbatim, Lines: lines[i : i+1], Start: i + 1})
			i++
		default:
			block, next := proseBlock(lines, i)
			blocks = append(blocks, block)
			i = next
		}
	}
	return blocks
}

// proseBlock consumes one paragraph or list item starting at lines[i].
func proseBlock(lines []string, i int) (Block, int) {
	indent, marker := listMarker(lines[i])
	block := Block{Kind: Prose, Start: i + 1, Marker: marker, Indent: indent}
	block.Lines = append(block.Lines, lines[i])
	for j := i + 1; j < len(lines); j++ {
		next := lines[j]
		if endsProse(next) {
			return block, j
		}
		// A new list item ends the previous one rather than continuing it.
		if _, m := listMarker(next); m != "" {
			return block, j
		}
		block.Lines = append(block.Lines, next)
	}
	return block, len(lines)
}

// endsProse reports a line that closes the paragraph before it. Indentation is
// absent from the test on purpose: inside a paragraph an indented line is the
// author's wrap, not a code block.
func endsProse(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || fenceDelimiter(line) != "" {
		return true
	}
	return strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "|") ||
		strings.HasPrefix(trimmed, "<!--") ||
		strings.HasPrefix(trimmed, "---")
}

// isVerbatimLine reports a line no prose rule may touch on its own.
func isVerbatimLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	switch {
	case trimmed == "":
		return true
	case strings.HasPrefix(trimmed, "#"):
		return true
	case strings.HasPrefix(trimmed, "|"):
		return true
	case strings.HasPrefix(trimmed, "<!--"), strings.HasPrefix(trimmed, "---"):
		return true
	}
	// An indented code block: four spaces or a tab, with no list marker.
	if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
		if _, marker := listMarker(line); marker == "" {
			return true
		}
	}
	return false
}

// fenceDelimiter returns the fence a line opens, empty when it opens none.
func fenceDelimiter(line string) string {
	trimmed := strings.TrimSpace(line)
	for _, delim := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, delim) {
			return delim
		}
	}
	return ""
}

// closingFence returns the index of the line that closes the fence opened at
// open, or the last line when the document never closes it.
func closingFence(lines []string, open int) int {
	delim := fenceDelimiter(lines[open])
	for j := open + 1; j < len(lines); j++ {
		if strings.HasPrefix(strings.TrimSpace(lines[j]), delim) {
			return j
		}
	}
	return len(lines) - 1
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
