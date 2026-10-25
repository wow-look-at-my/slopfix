// overlong_cuts.go finds the clause, sentence and paragraph boundaries a length repair cuts at.

package commentfix

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/table"
)

// clausesTable is what rules/ says for="comment-clauses".
var clausesTable = table.MustLoad(rules.FS, "comment-clauses")

// boundaryMarks answers the marks that end a clause.
func boundaryMarks() []string {
	for _, c := range clausesTable.Classes {
		if c.Name == "boundary" {
			return c.Words
		}
	}
	panic("commentfix: rules/ names no class boundary")
}

// clauseCuts answers every clause boundary in text, the last one first. A mark
// ends a clause when a space follows it. A word-length mark needs a space before it too.
// A tail opener, such as "so" or an open parenthesis, starts a part the cut can drop.
func clauseCuts(text string) []int {
	marks := boundaryMarks()
	tails := tailOpeners()
	var cuts []int
	for i := len(text) - 1; i > 0; i-- {
		for _, m := range marks {
			if !strings.HasPrefix(text[i:], m+" ") {
				continue
			}
			if len(m) > 1 && text[i-1] != ' ' {
				continue
			}
			cuts = append(cuts, i)
			break
		}
		if text[i-1] != ' ' {
			continue
		}
		for _, m := range tails {
			if strings.HasPrefix(text[i:], m) && (m == "(" || strings.HasPrefix(text[i+len(m):], " ")) {
				cuts = append(cuts, i)
				break
			}
		}
	}
	return cuts
}

// tailOpeners answers the words that open a part a cut can drop and leave the claim before it true.
func tailOpeners() []string {
	for _, c := range clausesTable.Classes {
		if c.Name == "tail" {
			return c.Words
		}
	}
	panic("commentfix: rules/ names no class tail")
}

// balanced reports text that closes every bracket, backtick and quote it opens.
func balanced(text string) bool {
	return strings.Count(text, "(") == strings.Count(text, ")") &&
		strings.Count(text, "[") == strings.Count(text, "]") &&
		strings.Count(text, "`")%2 == 0 && strings.Count(text, `"`)%2 == 0
}

// dangling words open something that must follow them, so a comment cannot end on one.
var dangling = set.Of(danglingWords()...)

// cutLastThought drops the last thought out of a block, and reports false when
// nothing is left to drop.
func cutLastThought(text []string) ([]string, bool) {
	if next, ok := dropParagraph(text); ok && endsWell(next) {
		return next, true
	}
	if next, ok := dropTrailingSentence(text); ok {
		return next, true
	}
	if next, ok := dropSentence(text); ok {
		return next, true
	}
	// Prose with no sentence end anywhere has no cut that reads.
	return text, false
}

// endsWell reports a block whose last line closes a sentence.
func endsWell(text []string) bool {
	return len(text) > 0 && endsSentence(text[len(text)-1])
}

// dropTrailingSentence removes the last sentence of the last paragraph and
// reflows what is left, so a cut lands mid-line where the prose ends there.
//
// It reports false for a block whose shape it cannot read. It also reports
// false for a last paragraph with no interior sentence end. dropParagraph and
// dropSentence own those.
func dropTrailingSentence(text []string) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, false
	}
	paras := paragraphs(text)
	last := -1
	for i, para := range paras {
		// A code block holds no sentence to drop, so the cut looks past it.
		if !para.blank && !para.verbatim {
			last = i
		}
	}
	if last < 0 {
		return text, false
	}

	// A comment sentence can open on an identifier in lower case, so the cut reads its ends as a comment does.
	body := strings.Join(paras[last].lines, " ")
	found := cutSentences(body)
	if len(found) < 2 {
		return text, false
	}
	kept := strings.TrimSpace(strings.Join(found[:len(found)-1], " "))
	if kept == "" {
		return text, false
	}

	var out []string
	for i, para := range paras {
		switch {
		case i > last:
			// Nothing follows the last prose paragraph but blank markers.
		case para.blank:
			out = append(out, indent+marker)
		case para.verbatim:
			out = append(out, para.raw...)
		case i == last:
			out = append(out, reflow(kept, indent, marker, wrapWidth)...)
		default:
			for _, line := range para.lines {
				out = append(out, indent+marker+" "+line)
			}
		}
	}
	if len(out) == 0 {
		return text, false
	}
	return out, true
}

// dropSentence removes the trailing lines back to the last sentence that ends,
// and reports false when the run holds no earlier ending.
func dropSentence(text []string) ([]string, bool) {
	for i := len(text) - 1; i > 0; i-- {
		if endsSentence(text[i-1]) {
			return text[:i], true
		}
	}
	return text, false
}

// endsSentence reports a comment line whose prose closes. It reads past a
// closing bracket or quote, so a line ending `... (see above).` counts.
func endsSentence(line string) bool {
	if bannerTitle(line) {
		return true
	}
	t := strings.TrimSpace(line)
	t = strings.TrimSpace(strings.TrimSuffix(t, "*/"))
	t = strings.TrimRight(t, `)]}"'`+"`")
	if t == "" {
		return false
	}
	switch t[len(t)-1] {
	case '.', '!', '?':
		// An ellipsis or an abbreviation is not the end of a thought.
		return !strings.HasSuffix(t, "..") && !strings.HasSuffix(t, "e.g.") && !strings.HasSuffix(t, "i.e.")
	}
	return false
}

// bannerRule is a section title between runs of rule marks, as in "// ---- dataset loading ----".
var bannerRule = regexp.MustCompile(`^\s*(?://+|#+)\s*[-=~*]{3,}\s+.*[A-Za-z].*\s[-=~*]{3,}\s*$`)

// bannerTitle reports a comment line that is a section title.
func bannerTitle(line string) bool {
	return bannerRule.MatchString(line)
}

// dropParagraph removes the last blank-separated paragraph, and reports false when no break remains.
func dropParagraph(text []string) ([]string, bool) {
	for i := len(text) - 1; i > 0; i-- {
		if isBlankComment(text[i]) {
			return text[:i], true
		}
	}
	return text, false
}

// isBlankComment reports a comment line carrying no prose, which is how a
// comment block spells a paragraph break.
func isBlankComment(line string) bool {
	t := strings.TrimSpace(line)
	for _, marker := range []string{"//!", "//", "#", "*"} {
		if t == marker {
			return true
		}
		if rest, found := strings.CutPrefix(t, marker); found && strings.TrimSpace(rest) == "" {
			return true
		}
	}
	return t == ""
}

// opening is the block's leading line of prose, bounded so a report quotes a recognisable fragment.
func opening(text []string) string {
	for _, line := range text {
		t := strings.TrimSpace(line)
		if isBlankComment(line) {
			continue
		}
		if len(t) > 90 {
			t = t[:87] + "..."
		}
		return t
	}
	return ""
}

func splitLines(src string) []string { return strings.Split(src, "\n") }
