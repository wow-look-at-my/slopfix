// Package prose splits a document into the parts a style rule may read, and
// applies the rules to those parts only.
package prose

import "strings"

// Kind is what a segment of a document is.
type Kind int

const (
	// KindProse is text a style rule reads.
	KindProse Kind = iota
	// KindFence is a fenced code block, including both fence lines.
	KindFence
	// KindIndentedCode is a block indented by four spaces or one tab.
	KindIndentedCode
	// KindTable is a pipe table, including its delimiter row.
	KindTable
	// KindHeading is an ATX heading line.
	KindHeading
	// KindFrontmatter is the YAML block a document may open with.
	KindFrontmatter
	// KindBlank is one or more empty lines.
	KindBlank
)

// Exempt reports whether a rule must skip this kind.
//
// A code block is data. A style guard that fires inside one has misread the
// block, and the answer is never to delete the block.
func (k Kind) Exempt() bool {
	switch k {
	case KindFence, KindIndentedCode, KindTable, KindFrontmatter, KindBlank:
		return true
	default:
		return false
	}
}

// Segment is one run of lines of a single kind.
type Segment struct {
	Kind  Kind
	Line  int // 1-based line number of the first line
	Lines []string
}

// Unit is one prose paragraph, list item, or blockquote. A rule that measures
// a sentence or a wrapped line reads a unit, never a whole segment.
type Unit struct {
	Line  int // 1-based line number of the first line
	Lines []string
	List  bool // the unit is a list item or a blockquote line
}

// Text joins the unit's lines with single spaces.
func (u Unit) Text() string {
	return strings.Join(u.Lines, " ")
}

const fenceMin = 3

// Segment splits a document into segments.
func Segment(src string) []Segment {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var segs []Segment
	i := 0

	if end, ok := frontmatterEnd(lines); ok {
		segs = append(segs, Segment{Kind: KindFrontmatter, Line: 1, Lines: lines[:end+1]})
		i = end + 1
	}

	for i < len(lines) {
		line := lines[i]

		if strings.TrimSpace(line) == "" {
			start := i
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}
			segs = append(segs, Segment{Kind: KindBlank, Line: start + 1, Lines: lines[start:i]})
			continue
		}

		if marker, ok := fenceOpen(line); ok {
			start := i
			i++
			for i < len(lines) && !fenceCloses(lines[i], marker) {
				i++
			}
			if i < len(lines) {
				i++ // the closing fence belongs to the block
			}
			segs = append(segs, Segment{Kind: KindFence, Line: start + 1, Lines: lines[start:i]})
			continue
		}

		if strings.HasPrefix(strings.TrimLeft(line, " "), "#") && leadingSpaces(line) < 4 {
			segs = append(segs, Segment{Kind: KindHeading, Line: i + 1, Lines: lines[i : i+1]})
			i++
			continue
		}

		if isIndentedCode(lines, i, segs) {
			start := i
			for i < len(lines) && (isIndented(lines[i]) || strings.TrimSpace(lines[i]) == "") {
				i++
			}
			for i > start && strings.TrimSpace(lines[i-1]) == "" {
				i-- // a trailing blank line belongs to the blank segment
			}
			segs = append(segs, Segment{Kind: KindIndentedCode, Line: start + 1, Lines: lines[start:i]})
			continue
		}

		if tableStarts(lines, i) {
			start := i
			for i < len(lines) && strings.Contains(lines[i], "|") {
				i++
			}
			segs = append(segs, Segment{Kind: KindTable, Line: start + 1, Lines: lines[start:i]})
			continue
		}

		start := i
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			if _, ok := fenceOpen(lines[i]); ok && i > start {
				break
			}
			if tableStarts(lines, i) && i > start {
				break
			}
			i++
		}
		segs = append(segs, Segment{Kind: KindProse, Line: start + 1, Lines: lines[start:i]})
	}

	return segs
}

// Units splits a prose segment into the paragraphs and list items a rule
// measures one at a time.
func Units(seg Segment) []Unit {
	if seg.Kind != KindProse {
		return nil
	}
	var units []Unit
	cur := Unit{Line: seg.Line}
	for n, line := range seg.Lines {
		if listMarker(line) || blockquote(line) {
			if len(cur.Lines) > 0 {
				units = append(units, cur)
			}
			cur = Unit{Line: seg.Line + n, Lines: []string{line}, List: true}
			continue
		}
		if len(cur.Lines) == 0 {
			cur.Line = seg.Line + n
		}
		cur.Lines = append(cur.Lines, line)
	}
	if len(cur.Lines) > 0 {
		units = append(units, cur)
	}
	return units
}

func frontmatterEnd(lines []string) (int, bool) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i, true
		}
	}
	return 0, false
}

// fenceOpen reports the fence marker a line opens a code block with. The
// marker carries its own length, because a longer fence closes only on a fence
// at least as long. That is how a document quotes a fence inside a fence.
func fenceOpen(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if leadingSpaces(line) >= 4 {
		return "", false
	}
	for _, char := range []string{"`", "~"} {
		run := runLength(trimmed, char)
		if run >= fenceMin {
			return strings.Repeat(char, run), true
		}
	}
	return "", false
}

func fenceCloses(line, marker string) bool {
	trimmed := strings.TrimSpace(line)
	char := marker[:1]
	if runLength(trimmed, char) < len(marker) {
		return false
	}
	return strings.Trim(trimmed, char) == ""
}

func runLength(s, char string) int {
	n := 0
	for strings.HasPrefix(s[n:], char) {
		n++
	}
	return n
}

func leadingSpaces(line string) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

func isIndented(line string) bool {
	return strings.TrimSpace(line) != "" && leadingSpaces(line) >= 4
}

// isIndentedCode reports whether an indented line opens a code block rather
// than continuing a list item. A list item's continuation is indented too, so
// the previous segment decides.
func isIndentedCode(lines []string, i int, segs []Segment) bool {
	if !isIndented(lines[i]) {
		return false
	}
	for n := len(segs) - 1; n >= 0; n-- {
		switch segs[n].Kind {
		case KindBlank:
			continue
		case KindProse:
			return !hasListItem(segs[n])
		default:
			return true
		}
	}
	return true
}

func hasListItem(seg Segment) bool {
	for _, line := range seg.Lines {
		if listMarker(line) {
			return true
		}
	}
	return false
}

func tableStarts(lines []string, i int) bool {
	if !strings.Contains(lines[i], "|") || i+1 >= len(lines) {
		return false
	}
	next := strings.TrimSpace(lines[i+1])
	if !strings.Contains(next, "|") || !strings.Contains(next, "-") {
		return false
	}
	return strings.Trim(next, "|-: \t") == ""
}

func listMarker(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")
	for _, prefix := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	digits := 0
	for digits < len(trimmed) && trimmed[digits] >= '0' && trimmed[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits+1 >= len(trimmed) {
		return false
	}
	return (trimmed[digits] == '.' || trimmed[digits] == ')') && trimmed[digits+1] == ' '
}

func blockquote(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), ">")
}
