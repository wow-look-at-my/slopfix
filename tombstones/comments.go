// Package tombstones finds a comment that describes a state the code is no
// longer in, or that argues for the diff instead of telling the next editor
// what breaks.
//
// Only prose is judged. In source that is the comments, never the code, so a
// string literal holding the word "previously" is not a tombstone. In a
// document it is the prose, with fences and frontmatter skipped.
//
// The input may be a fragment rather than a whole file, so a scanner starting
// mid-string can read code as a comment. That costs a false positive at worst.
package tombstones

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/wow-look-at-my/slopfix/commentfix"
)

// Block is a comment run or a paragraph. LineNos and Pure place each line and
// judge it, and are nil for a paragraph.
type Block struct {
	Text    string
	Lines   int
	LineNos []int
	Pure    []bool
}

// style says how a language spells a comment.
type style struct {
	line       []string
	blockOpen  string
	blockClose string
	strings    []byte // quote characters that start a literal
	raw        byte   // a quote character whose literal ignores backslashes
}

var (
	cLike   = style{line: []string{"//"}, blockOpen: "/*", blockClose: "*/", strings: []byte{'"', '\''}, raw: '`'}
	hashish = style{line: []string{"#"}, strings: []byte{'"', '\''}}
	dashish = style{line: []string{"--"}, strings: []byte{'"', '\''}}
	lispish = style{line: []string{";"}, strings: []byte{'"'}}
)

// byExt maps a file extension to its comment spelling. An extension absent from
// this table is not judged at all: guessing at a syntax risks reading code as
// prose, and a verdict built on that is the kind a user turns off.
var byExt = map[string]style{
	".go": cLike, ".c": cLike, ".h": cLike, ".cc": cLike, ".cpp": cLike,
	".hpp": cLike, ".hh": cLike, ".cxx": cLike, ".m": cLike, ".mm": cLike,
	".java": cLike, ".js": cLike, ".mjs": cLike, ".cjs": cLike, ".jsx": cLike,
	".ts": cLike, ".tsx": cLike, ".mts": cLike, ".cts": cLike, ".rs": cLike,
	".swift": cLike, ".kt": cLike, ".kts": cLike, ".cs": cLike, ".scala": cLike,
	".php": cLike, ".dart": cLike, ".zig": cLike, ".glsl": cLike, ".wgsl": cLike,
	".vert": cLike, ".frag": cLike, ".comp": cLike, ".proto": cLike, ".jq": cLike,

	".sh": hashish, ".bash": hashish, ".zsh": hashish, ".fish": hashish,
	".rb": hashish, ".pl": hashish, ".r": hashish, ".yml": hashish,
	".yaml": hashish, ".toml": hashish, ".tf": hashish, ".just": hashish,
	".dats": hashish, ".cmake": hashish, ".mk": hashish,

	".sql": dashish, ".lua": dashish, ".hs": dashish, ".elm": dashish,
	".ads": dashish, ".adb": dashish,

	".el": lispish, ".lisp": lispish, ".clj": lispish, ".scm": lispish,
}

// byBase covers the extensionless files that still carry comments.
var byBase = map[string]style{
	"makefile": hashish, "dockerfile": hashish, "justfile": hashish,
	"containerfile": hashish, ".gitignore": hashish, ".dockerignore": hashish,
}

// AddedBlocks returns the prose that added contributes to path. It returns nil
// for a path this package does not judge.
func AddedBlocks(path, added string) []Block {
	if IsDocument(path) {
		return paragraphs(added)
	}
	st, ok := styleFor(path)
	if !ok {
		return nil
	}
	return commentBlocks(path, added, st)
}

func styleFor(path string) (style, bool) {
	base := strings.ToLower(filepath.Base(path))
	if st, ok := byBase[base]; ok {
		return st, true
	}
	st, ok := byExt[strings.ToLower(filepath.Ext(path))]
	return st, ok
}

// IsDocument reports whether path names prose rather than source.
func IsDocument(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".rst", ".txt", ".adoc":
		return true
	}
	return false
}

// piece is a comment spanning physical lines. firstPure and lastPure judge
// its opening and closing line.
type piece struct {
	line      int
	text      string
	firstPure bool
	lastPure  bool
}

// commentBlocks collects each comment in src. It merges a run of adjacent line
// comments, so the volume cap sees the essay.
//
// A grammar answers where the comments are wherever one reads the language. The
// scan below is what is left: it reads markers out of the raw text, and a
// marker inside a string literal spanning several lines reads as prose there.
func commentBlocks(path, src string, st style) []Block {
	pieces, ok := treePieces(path, src, st)
	if !ok {
		pieces = scanPieces(src, st)
	}
	return groupPieces(pieces)
}

// treePieces answers the comments a grammar found, and whether it read the
// language at all. A comment node carries its own span, so nothing quoted can
// be mistaken for prose.
func treePieces(path, src string, st style) ([]piece, bool) {
	if !commentfix.Parsed(path) {
		return nil, false
	}
	starts := lineStarts(src)
	lines := strings.Split(src, "\n")
	var out []piece
	for _, c := range commentfix.Comments(path, src) {
		first := lineIndex(starts, c.Offset)
		last := lineIndex(starts, c.Offset+len(c.Text))
		if first >= len(lines) || last >= len(lines) {
			continue
		}
		// A block comment's piece is what the delimiters enclose, which is
		// what the scan records and what the grouping below counts lines of.
		text := c.Text
		if st.blockOpen != "" && strings.HasPrefix(text, st.blockOpen) {
			text = strings.TrimSuffix(strings.TrimPrefix(text, st.blockOpen), st.blockClose)
		}
		out = append(out, piece{
			line:      first,
			text:      text,
			firstPure: strings.TrimSpace(lines[first][:c.Offset-starts[first]]) == "",
			lastPure:  strings.TrimSpace(lines[last][c.Offset+len(c.Text)-starts[last]:]) == "",
		})
	}
	return out, true
}

// lineStarts answers the byte offset each line of src begins at.
func lineStarts(src string) []int {
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineIndex answers which line an offset falls on, counting from zero.
func lineIndex(starts []int, offset int) int {
	n := sort.SearchInts(starts, offset+1) - 1
	if n < 0 {
		return 0
	}
	return n
}

// scanPieces reads comment markers out of the raw text, for a language no
// grammar here parses.
func scanPieces(src string, st style) []piece {
	var pieces []piece

	lines := strings.Split(src, "\n")
	inBlock := false
	var block []string
	blockStart := 0
	blockFirstPure := false

	for n, line := range lines {
		rest := line
		piecesThisLine := 0
		if inBlock {
			if i := strings.Index(rest, st.blockClose); i >= 0 {
				block = append(block, rest[:i])
				after := rest[i+len(st.blockClose):]
				pieces = append(pieces, piece{
					line:      blockStart,
					text:      strings.Join(block, "\n"),
					firstPure: blockFirstPure,
					lastPure:  strings.TrimSpace(after) == "",
				})
				piecesThisLine++
				block, inBlock = nil, false
				rest = after
			} else {
				block = append(block, rest)
				continue
			}
		}
		for {
			at, kind := nextMarker(rest, st)
			if at < 0 {
				break
			}
			// A comment sharing this raw line makes purity ambiguous.
			firstOnLine := piecesThisLine == 0
			if kind == markerBlock {
				open := rest[at+len(st.blockOpen):]
				if i := strings.Index(open, st.blockClose); i >= 0 {
					pieces = append(pieces, piece{
						line:      n,
						text:      open[:i],
						firstPure: firstOnLine && strings.TrimSpace(rest[:at]) == "",
						lastPure:  strings.TrimSpace(open[i+len(st.blockClose):]) == "",
					})
					piecesThisLine++
					rest = open[i+len(st.blockClose):]
					continue
				}
				inBlock, blockStart, block = true, n, []string{open}
				blockFirstPure = firstOnLine && strings.TrimSpace(rest[:at]) == ""
			} else {
				pieces = append(pieces, piece{
					line:      n,
					text:      rest[at:],
					firstPure: firstOnLine && strings.TrimSpace(rest[:at]) == "",
					lastPure:  true, // a line comment always runs to end of line
				})
				piecesThisLine++
			}
			break
		}
	}
	if inBlock {
		pieces = append(pieces, piece{
			line:      blockStart,
			text:      strings.Join(block, "\n"),
			firstPure: blockFirstPure,
			lastPure:  false, // never closed: the text ended mid-comment
		})
	}

	return pieces
}

// groupPieces merges a run of adjacent comments into one block.
func groupPieces(pieces []piece) []Block {
	var out []Block
	for i := 0; i < len(pieces); {
		j := i
		text := []string{pieces[i].text}
		lineNos, pure := pieceLines(pieces[i])
		height := strings.Count(pieces[i].text, "\n") + 1
		for j+1 < len(pieces) && pieces[j+1].line == pieces[j].line+1 {
			j++
			text = append(text, pieces[j].text)
			l, p := pieceLines(pieces[j])
			lineNos = append(lineNos, l...)
			pure = append(pure, p...)
			height += strings.Count(pieces[j].text, "\n") + 1
		}
		out = append(out, Block{Text: strings.Join(text, "\n"), Lines: height, LineNos: lineNos, Pure: pure})
		i = j + 1
	}
	return out
}

// pieceLines expands a piece into the per-line arrays a Block carries.
func pieceLines(p piece) (lineNos []int, pure []bool) {
	n := strings.Count(p.text, "\n") + 1
	lineNos = make([]int, n)
	pure = make([]bool, n)
	for i := range lineNos {
		lineNos[i] = p.line + i
	}
	if n == 1 {
		pure[0] = p.firstPure && p.lastPure
		return lineNos, pure
	}
	pure[0] = p.firstPure
	for i := 1; i < n-1; i++ {
		pure[i] = true // fully inside the comment; nothing else can share it
	}
	pure[n-1] = p.lastPure
	return lineNos, pure
}

const (
	markerLine = iota
	markerBlock
)

// nextMarker finds where the next comment starts in a line of code, skipping
// string literals so a quoted marker stays a literal.
func nextMarker(line string, st style) (int, int) {
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == st.raw && st.raw != 0 {
			if j := strings.IndexByte(line[i+1:], st.raw); j >= 0 {
				i += j + 1
				continue
			}
			return -1, 0
		}
		if isQuote(c, st.strings) {
			j := skipString(line, i, c)
			if j < 0 {
				return -1, 0
			}
			i = j
			continue
		}
		if st.blockOpen != "" && strings.HasPrefix(line[i:], st.blockOpen) {
			return i, markerBlock
		}
		for _, m := range st.line {
			if strings.HasPrefix(line[i:], m) {
				return i, markerLine
			}
		}
	}
	return -1, 0
}

func isQuote(c byte, quotes []byte) bool {
	for _, q := range quotes {
		if c == q {
			return true
		}
	}
	return false
}

// skipString returns the index of the closing quote. It reports a negative
// index when the literal runs past the end of the line.
func skipString(line string, start int, quote byte) int {
	for i := start + 1; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case quote:
			return i
		}
	}
	return -1
}

// paragraphs splits a document into blank-line-separated blocks. It drops what
// is not the document's own voice: code, HTML comments and frontmatter.
func paragraphs(doc string) []Block {
	lines := strings.Split(doc, "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}

	var out []Block
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, Block{Text: strings.Join(cur, "\n"), Lines: len(cur)})
			cur = nil
		}
	}
	inFence, inComment := false, false
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			inFence = !inFence
			flush()
			continue
		case inFence:
			continue
		case strings.Contains(line, "<!--"):
			inComment = !strings.Contains(line, "-->")
			continue
		case inComment:
			inComment = !strings.Contains(line, "-->")
			continue
		case strings.HasPrefix(line, "    "), strings.HasPrefix(line, "\t"):
			continue
		case trimmed == "":
			flush()
			continue
		}
		cur = append(cur, blankInlineCode(line))
	}
	flush()
	return out
}

// blankInlineCode replaces each backtick span with spaces, keeping every byte
// offset. A phrase inside verbatim machinery is a literal, not a claim.
func blankInlineCode(line string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(line); i++ {
		if line[i] == '`' {
			in = !in
			b.WriteByte(' ')
			continue
		}
		if in {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(line[i])
	}
	return b.String()
}
