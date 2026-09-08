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
	commented := commentLineNumbers(filename, src)
	if len(commented) == 0 {
		return Repair{Text: src}
	}

	lines := strings.Split(src, "\n")
	repaired, removed, blanked := repairLines(lines, commented)
	out := dropEmptied(strings.Join(repaired, "\n"), blanked)
	return Repair{Text: out, Changed: out != src, Removed: removed}
}

// commentLineNumbers reports where a comment starts on each line carrying it,
// as a byte offset into the line. The parser answers where the comments are, so
// a marker inside a string literal is left alone.
//
// A comment that opens partway along a line follows code, and the offset is
// what keeps that code out of the rewrite.
func commentLineNumbers(filename, src string) map[int]int {
	out := make(map[int]int)
	for _, c := range treecomments.Extract(filename, src) {
		at := 1 + strings.Count(src[:c.Offset], "\n")
		out[at] = c.Offset - (strings.LastIndexByte(src[:c.Offset], '\n') + 1)
		for i := range strings.Count(c.Text, "\n") {
			out[at+i+1] = 0
		}
	}
	return out
}

// repairLines rewrites a file's comments a paragraph at a time. It returns the
// repaired lines, the sentences it cut, and the lines it left with nothing to
// say.
//
// A paragraph rather than a line, because a sentence wraps: cutting the share
// a line carries leaves the rest of that sentence dangling below it.
func repairLines(lines []string, commented map[int]int) (repaired []string, removed []string, blanked map[int]bool) {
	blanked = make(map[int]bool)
	for _, para := range paragraphsOf(lines, commented) {
		said := Say(para.prose)
		said, cut := cutWhatIsLeft(said)
		removed = append(removed, cut...)
		if said == para.prose {
			continue
		}
		if said == "" && para.code != "" {
			// A comment following code loses the comment, and the code stays.
			lines[para.lines[0]] = strings.TrimRight(para.code, " \t")
			continue
		}
		wrapped := wrap(said, para.marker, para.width)
		for i, at := range para.lines {
			if i < len(wrapped) {
				lines[at] = wrapped[i]
				continue
			}
			// Fewer lines than before: the rest go bare, and the caller drops them.
			blanked[at+1] = true
			lines[at] = strings.TrimRight(para.marker, " ")
		}
		if len(wrapped) > len(para.lines) {
			// It does not fit: the tail joins the last line, so no offset moves.
			last := para.lines[len(para.lines)-1]
			lines[last] = strings.Join(append([]string{lines[last]}, proseOf(wrapped[len(para.lines):])...), " ")
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
}

// paragraphsOf groups a comment's lines into the paragraphs a rewrite acts on.
// A directive, a blank comment line and a line carrying no marker all break the
// run: none of them is prose a sentence continues through.
func paragraphsOf(lines []string, commented map[int]int) []para {
	var out []para
	var current *para
	for i, line := range lines {
		at, isComment := commented[i+1]
		if !isComment || at > len(line) {
			current = nil
			continue
		}
		marker, prose, ok := split(line[at:])
		marker = line[:at] + marker
		if !ok || prose == "" || isDirective(line[at:]) {
			current = nil
			continue
		}
		if at > 0 {
			// A comment following code stands alone and cannot be rewrapped.
			out = append(out, para{marker: marker, lines: []int{i}, prose: prose, width: len(line), code: line[:at]})
			current = nil
			continue
		}
		if current == nil || current.marker != marker {
			out = append(out, para{marker: marker})
			current = &out[len(out)-1]
		}
		current.lines = append(current.lines, i)
		current.prose = strings.TrimSpace(current.prose + " " + prose)
		current.width = max(current.width, len(line))
	}
	return out
}

// wrap lays prose back onto comment lines at the width the paragraph had. A
// word longer than the width goes on its own line rather than being broken: a
// URL or an identifier split across lines stops being either.
func wrap(prose, marker string, width int) []string {
	words := strings.Fields(prose)
	if len(words) == 0 {
		return nil
	}
	var out []string
	line := marker
	for _, w := range words {
		if line != marker && len(line)+len(w) > width {
			out = append(out, strings.TrimRight(line, " "))
			line = marker
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

// split separates a comment line's marker and indent from its prose. It reports
// false for a line carrying no marker, such as the middle of a block comment.
func split(line string) (marker, prose string, ok bool) {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	for _, m := range []string{"///", "//", "#", "*"} {
		rest, found := strings.CutPrefix(trimmed, m)
		if !found {
			continue
		}
		space := rest[:len(rest)-len(strings.TrimLeft(rest, " \t"))]
		return indent + m + space, strings.TrimSpace(rest), true
	}
	return "", "", false
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
