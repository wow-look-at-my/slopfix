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
	"github.com/wow-look-at-my/slopfix/source"
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
	comments := source.Extract(filename, src)
	if len(comments) == 0 {
		return Repair{Text: src}
	}

	out := src
	var removed []string
	emptied := make(map[int]bool)
	// Back to front, so an earlier comment's offset stays valid.
	for i := len(comments) - 1; i >= 0; i-- {
		c := comments[i]
		repaired, cut, blanked := repairComment(c.Text)
		if repaired == c.Text {
			continue
		}
		removed = append(removed, cut...)
		line := 1 + strings.Count(src[:c.Offset], "\n")
		for _, n := range blanked {
			emptied[line+n] = true
		}
		out = out[:c.Offset] + repaired + out[c.Offset+len(c.Text):]
	}

	out = dropEmptied(out, emptied)
	reverse(removed)
	return Repair{Text: out, Changed: out != src, Removed: removed}
}

// repairComment rewrites a comment token, line by line. It returns the repaired
// token, the sentences it cut, and the lines it left with nothing to say.
func repairComment(text string) (repaired string, removed []string, blanked []int) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if isDirective(line) {
			continue
		}
		marker, prose, ok := split(line)
		if !ok || prose == "" {
			continue
		}
		said := Say(prose)
		said, cut := cutWhatIsLeft(said)
		removed = append(removed, cut...)
		if said == prose {
			continue
		}
		if said == "" {
			blanked = append(blanked, i)
			lines[i] = strings.TrimRight(marker, " ")
			continue
		}
		lines[i] = marker + said
	}
	return strings.Join(lines, "\n"), removed, blanked
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

// reverse puts the removed sentences back into source order, which the walk
// above collects them out of.
func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
