// repair.go is the repair half of the rule: what a comment says instead of the
// number it states.
//
// The table says it in words wherever a swap keeps the meaning. What no entry
// covers is not guessed at, because nobody reviews what a repair applied: the
// sentence carrying the number is cut, and the caller is told what went. A
// comment left with nothing to say loses its line.
package commentnumbers

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// Repair is the source with the numbers rewritten out of its comments.
type Repair struct {
	// Text is the repaired source. It equals the input when Changed is false.
	Text string `json:"text"`
	// Changed reports whether any rewrite applied.
	Changed bool `json:"changed"`
	// Removed quotes each sentence the repair cut, because no entry covered it.
	Removed []string `json:"removed,omitempty"`
}

// Fix rewrites every number a comment states and returns the repaired source.
//
// A generated file is left alone, and so is a language the extractor has no
// syntax for: both report no findings, so both have nothing to repair.
func Fix(filename, src string) Repair {
	if IsGenerated(src) {
		return Repair{Text: src}
	}
	runs := treecomments.Runs(filename, src)
	if len(runs) == 0 {
		return Repair{Text: src}
	}

	lines := strings.Split(src, "\n")
	repaired, removed, blanked := repairRuns(lines, runs)
	out := dropEmptied(strings.Join(repaired, "\n"), blanked)
	return Repair{Text: out, Changed: out != src, Removed: removed}
}

// repairRuns rewrites a file's comments a run at a time. It returns the
// repaired lines, the sentences it cut, and the lines it left with nothing to
// say.
//
// A run rather than a line, because a sentence wraps: cutting the share a line
// carries leaves the rest of that sentence dangling below it.
func repairRuns(lines []string, runs []treecomments.Run) (repaired []string, removed []string, blanked map[int]bool) {
	blanked = make(map[int]bool)
	for _, para := range paragraphsOf(lines, runs) {
		said := Say(para.prose)
		said, cut := cutWhatIsLeft(said)
		removed = append(removed, cut...)
		if said == para.prose {
			continue
		}
		if said == "" && para.code != "" && para.trailer == "" {
			// A comment following code loses the comment, and the code stays.
			lines[para.lines[0]] = strings.TrimRight(para.code, " \t")
			continue
		}
		wrapped := wrap(said, para.marker, para.cont, para.width)
		// An emptied block keeps its delimiters on a line of their own.
		if len(wrapped) == 0 && para.trailer != "" {
			wrapped = []string{strings.TrimRight(para.marker, " ")}
		}
		// The closer goes on last, after the line juggling below has settled which line IS last. Carried inside wrapped it
		closeAt := -1
		for i, at := range para.lines {
			if i < len(wrapped) {
				lines[at] = wrapped[i]
				closeAt = at
				continue
			}
			// Fewer lines than before: the rest go bare, and the caller drops them.
			blanked[at+1] = true
			lines[at] = strings.TrimRight(para.cont, " ")
		}
		if len(wrapped) > len(para.lines) {
			// It does not fit: the tail joins the last line, so no offset moves.
			last := para.lines[len(para.lines)-1]
			lines[last] = strings.Join(append([]string{lines[last]}, proseOf(wrapped[len(para.lines):])...), " ")
			closeAt = last
		}
		if para.trailer != "" && closeAt >= 0 {
			lines[closeAt] = strings.TrimRight(lines[closeAt], " ") + " " + para.trailer
		}
	}
	return lines, removed, blanked
}

// para is a run of comment lines carrying a single paragraph of prose.
type para struct {
	// lines are the indexes the paragraph occupies, in order.
	lines []int
	// marker is the indent and comment marker its lines share.
	marker string
	// prose is the paragraph, joined.
	prose string
	// width is the longest line it already used, so a rewrite wraps as it did.
	width int
	// code is what sits before a comment that follows code on its line.
	code string
	// cont is the marker a continuation line carries.
	cont string
	// trailer closes a block comment, and rides the last line a rewrite emits.
	trailer string
}

// paragraphsOf turns the parser's runs into the paragraphs a rewrite acts on. A
// run breaks further on a directive and on a blank comment line: a directive
// addresses a tool, and a blank line is a break somebody wrote.
func paragraphsOf(lines []string, runs []treecomments.Run) []para {
	var out []para
	for _, run := range runs {
		var current *para
		for _, c := range run {
			// A block comment is a single token spanning its lines.
			if isBlock(c.Text) && !isDirective(c.Text) {
				if b, ok := blockPara(lines, c); ok {
					out = append(out, b)
				}
				current = nil
				continue
			}
			for n := range max(c.Lines, 1) {
				i := c.Line - 1 + n
				if i < 0 || i >= len(lines) {
					continue
				}
				col := 0
				if n == 0 {
					col = c.Col
				}
				line := lines[i]
				marker, prose, trailer, ok := splitBlock(line[min(col, len(line)):])
				marker = line[:min(col, len(line))] + marker
				if !ok || (prose == "" && trailer == "") || isDirective(c.Text) {
					current = nil
					continue
				}
				if n == 0 && col > 0 && col > indentOf(line) {
					// A comment following code stands alone and cannot be rewrapped.
					out = append(out, para{marker: marker, cont: marker, lines: []int{i}, prose: prose, width: len(line), code: line[:col], trailer: trailer})
					current = nil
					continue
				}
				if current == nil {
					out = append(out, para{marker: marker, cont: marker})
					current = &out[len(out)-1]
				} else if len(current.lines) == 1 {
					// The next line shows what a continuation looks like.
					current.cont = marker
				}
				current.lines = append(current.lines, i)
				current.prose = strings.TrimSpace(current.prose + " " + prose)
				current.width = max(current.width, len(line))
				// The closer sits on the paragraph's last line, wherever that lands.
				if trailer != "" {
					current.trailer = trailer
				}
			}
		}
	}
	return out
}

// isBlock reports whether text opens a block comment.
func isBlock(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "/*")
}

// blockPara reads a whole block comment as a single paragraph, delimiters and
// all. A blank line inside it is a break in the prose, not a break in the
// comment, so a rewrite that honored it left the block unclosed.
func blockPara(lines []string, c treecomments.Comment) (para, bool) {
	b := para{}
	for n := range max(c.Lines, 1) {
		i := c.Line - 1 + n
		if i < 0 || i >= len(lines) {
			return b, false
		}
		col := 0
		if n == 0 {
			col = c.Col
		}
		line := lines[i]
		marker, prose, trailer, ok := splitBlock(line[min(col, len(line)):])
		if !ok {
			// A line inside the block carrying no marker at all: an indented example, or a table. A rewrap would destroy it, so
			return b, false
		}
		marker = line[:min(col, len(line))] + marker
		switch {
		case n == 0:
			b.marker, b.cont = marker, marker
			if col > 0 && col > indentOf(line) {
				b.code = line[:col]
			}
		case len(b.lines) == 1:
			b.cont = marker
		}
		b.lines = append(b.lines, i)
		if prose != "" {
			b.prose = strings.TrimSpace(b.prose + " " + prose)
		}
		b.width = max(b.width, len(line))
		if trailer != "" {
			b.trailer = trailer
		}
	}
	// A block with no closer is not a block this can safely rewrite.
	return b, b.prose != "" && b.trailer != ""
}

// indentOf reports the column a line's leading non-blank byte sits at.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// wrap lays prose back onto comment lines at the width the paragraph had. A
// word longer than the width goes on its own line rather than being broken: a
// URL or an identifier split across lines stops being either.
func wrap(prose, marker, cont string, width int) []string {
	words := strings.Fields(prose)
	if len(words) == 0 {
		return nil
	}
	var out []string
	line, at := marker, marker
	for _, w := range words {
		if line != at && len(line)+len(w) > width {
			out = append(out, strings.TrimRight(line, " "))
			at = cont
			line = cont
		}
		line += w + " "
	}
	return append(out, strings.TrimRight(line, " "))
}

// proseOf strips the marker off each wrapped line, for a tail that has to join
// the line above it.
func proseOf(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if _, prose, ok := split(line); ok {
			out = append(out, prose)
		}
	}
	return out
}

// cutWhatIsLeft removes the sentence around any number the table did not
// rewrite, and reports what it cut.
func cutWhatIsLeft(prose string) (string, []string) {
	if len(cardinal.Find(prose, cardinal.Comment)) == 0 {
		return prose, nil
	}
	var kept, cut []string
	for _, sentence := range sentences(prose) {
		if len(cardinal.Find(sentence, cardinal.Comment)) > 0 {
			cut = append(cut, strings.TrimSpace(sentence))
			continue
		}
		kept = append(kept, strings.TrimSpace(sentence))
	}
	return strings.TrimSpace(strings.Join(kept, " ")), cut
}

// sentences splits prose on its sentence ends, keeping the punctuation with the
// sentence it closes. A line ending mid-sentence counts as whole here, which is
// why a cut can take a wrapped line's share of the sentence it carries.
func sentences(prose string) []string {
	var out []string
	start := 0
	for i := 0; i < len(prose); i++ {
		if prose[i] != '.' && prose[i] != '!' && prose[i] != '?' {
			continue
		}
		if i+1 < len(prose) && prose[i+1] != ' ' {
			continue
		}
		out = append(out, prose[start:i+1])
		start = i + 1
	}
	if rest := strings.TrimSpace(prose[start:]); rest != "" {
		out = append(out, prose[start:])
	}
	return out
}

// split separates a comment line's marker and indent from its prose.
func split(line string) (marker, prose string, ok bool) {
	marker, prose, _, ok = splitBlock(line)
	return marker, prose, ok
}

// splitBlock is split, also reporting the delimiter that closes a block. The C
// family opens with a slash-star and closes with a star-slash, and the closer is
// not prose: left in the text a rewrite wraps it into the middle of the comment,
// and dropped it leaves the block open and the file unparseable.
func splitBlock(line string) (marker, prose, trailer string, ok bool) {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	for _, m := range []string{"///", "//", "/*", "#", "*"} {
		rest, found := strings.CutPrefix(trimmed, m)
		if !found {
			continue
		}
		space := rest[:len(rest)-len(strings.TrimLeft(rest, " \t"))]
		body := strings.TrimSpace(rest)
		if cut, held := strings.CutSuffix(body, "*/"); held {
			return indent + m + space, strings.TrimSpace(cut), "*/", true
		}
		return indent + m + space, body, "", true
	}
	return "", "", "", false
}

// dropEmptied removes the lines the repair left carrying a bare marker. A line
// the source already carried that way is left alone: a blank comment line is a
// paragraph break somebody wrote on purpose.
func dropEmptied(src string, emptied map[int]bool) string {
	if len(emptied) == 0 {
		return src
	}
	lines := strings.Split(src, "\n")
	kept := lines[:0]
	for i, line := range lines {
		if emptied[i+1] && bareMarker(line) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// bareMarker reports whether the line carries a comment marker and nothing else.
func bareMarker(line string) bool {
	switch strings.TrimSpace(line) {
	case "//", "///", "#", "*":
		return true
	}
	return false
}
