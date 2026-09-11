// Package commentlength finds a comment longer than the code it documents.
//
// A comment earns its place by stopping the next mistake. A comment that runs
// longer than the code becomes an essay, and the reader pays for it on every
// pass through the file. The rule is a proxy rather than a judgement of
// content: length is what a machine can measure.
//
// It reads a real syntax tree, so a span is exact rather than guessed, and the
// same code serves every grammar in treeblocks.go. Nothing here names a
// language.
//
// The repair is to cut, from the end. A comment leads with its point and
// elaborates afterwards, so the trailing paragraph is what a reader loses least
// by losing. The opening sentence is never cut: a block trimmed to nothing is a
// worse edit than a block left long.
package commentlength

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/ste"
)

// ID names this rule, on a report and on the command line alike.
const ID = "comments/length"

// floorChars is the size a comment may always be, whatever it documents.
const floorChars = 120

// Hit is a comment block that outweighs its code.
type Hit struct {
	// ID names the rule, the way a compiler names a warning.
	ID string `json:"id"`
	// Tell says in words which measure was exceeded.
	Tell string `json:"tell"`
	// Sentence quotes the comment's opening, so a report is recognisable.
	Sentence string `json:"sentence"`
	// Line is where the block starts, counting from the top of the file.
	Line int `json:"line"`
	// Repairable reports whether Fix fits the block without cutting its opening.
	Repairable bool `json:"repairable"`
}

// block is a run of comment lines and the code beneath it.
type block struct {
	// start and end are line indexes into the file, half open.
	start, end int
	// codeLines and codeChars measure what the block documents.
	codeLines, codeChars int
	// text is the comment's lines, marker and all.
	text []string
	// exact is true when a parser decided this span rather than a line walk. It gates the REPAIR and nothing else.
	exact bool
}

// Check reports every comment block in src that outweighs its code.
func Check(filename, src string) []Hit {
	var hits []Hit
	for _, b := range blocks(filename, src) {
		tell, over := judge(b)
		if !over {
			continue
		}
		hits = append(hits, Hit{
			ID:         ID,
			Tell:       tell,
			Sentence:   opening(b.text),
			Line:       b.start + 1,
			Repairable: b.exact && len(trim(b)) < len(b.text),
		})
	}
	return hits
}

// Fix cuts every over-long comment block back inside its budget, from the end,
// stopping before the opening sentence.
func Fix(filename, src string) (string, bool) {
	bs := blocks(filename, src)
	if len(bs) == 0 {
		return src, false
	}
	lines := splitLines(src)
	changed := false

	// Back to front, so an earlier block's line numbers stay valid.
	for i := len(bs) - 1; i >= 0; i-- {
		b := bs[i]
		if _, over := judge(b); !over {
			continue
		}
		// A guessed span is reported but never rewritten. Wrong by a line, it
		// deletes the wrong sentence, and nobody reviews what a hook applied.
		if !b.exact {
			continue
		}
		kept := trim(b)
		// A repair that keeps the line count still shortens the text, and the
		// character half of the rule is what it answers.
		if strings.Join(kept, "\n") == strings.Join(b.text, "\n") {
			continue
		}
		lines = append(lines[:b.start], append(kept, lines[b.end:]...)...)
		changed = true
	}
	if !changed {
		return src, false
	}
	return strings.Join(lines, "\n"), true
}

// judge measures a block against its code and names every measure it failed.
// Lines catch an essay; characters catch a dense paragraph.
func judge(b block) (string, bool) {
	lines, chars := measure(prose(b.text))
	// Nothing to weigh against.
	if b.codeLines == 0 {
		if lines == 0 {
			return "", false
		}
		return "the comment documents nothing", true
	}
	limit := max(floorChars, b.codeChars)

	var tells []string
	if lines > b.codeLines {
		tells = append(tells, "the comment runs more lines than the code it documents")
	}
	if chars > limit {
		tells = append(tells, "the comment runs longer than the code it documents")
	}
	if len(tells) == 0 {
		return "", false
	}
	return strings.Join(tells, ", and "), true
}

// measure counts the non-blank lines and the non-whitespace characters of a
// run of text, so indentation costs nothing and both counts compare directly.
func measure(text []string) (lines, chars int) {
	for _, line := range text {
		content := false
		for _, r := range line {
			if unicode.IsSpace(r) {
				continue
			}
			content = true
			chars++
		}
		if content {
			lines++
		}
	}
	return lines, chars
}

// prose drops the directive lines from a block. A build constraint is an
// instruction to a tool, so measuring it reports an essay nobody wrote.
func prose(text []string) []string {
	kept := make([]string, 0, len(text))
	for _, line := range text {
		if isDirective(line) {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

// isDirective reports a line a tool reads rather than a reader. The C family
// spells it with no space after the marker, and the hash family carries the
// interpreter line and the linter pragma.
func isDirective(line string) bool {
	t := strings.TrimSpace(line)
	for _, marker := range []string{"//", "#"} {
		rest, found := strings.CutPrefix(t, marker)
		if !found {
			continue
		}
		if marker == "#" && strings.HasPrefix(rest, "!") {
			return true // an interpreter line
		}
		name, _, hasColon := strings.Cut(rest, ":")
		if !hasColon || name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		// `//go:build` and `# shellcheck:` carry no space before the colon.
		return true
	}
	return false
}

// trim cuts the block's trailing prose until it fits, keeping the opening.
// A paragraph goes before a line does, and the opening paragraph always survives.
func trim(b block) []string {
	kept := b.text

	// Tighten before cutting. A padded comment fits after its filler is gone and
	// it is reflowed, and keeping the whole thought beats losing the last of it.
	if tightened, did := tighten(kept); did {
		if _, over := judge(block{text: tightened, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return tightened
		}
		kept = tightened
	}

	// The words can fit where the wrap does not. Laying them out at the budget's
	// own width drops none, which is what the character floor is for.
	if wider, did := widen(kept, max(floorChars, b.codeChars)); did {
		if _, over := judge(block{text: wider, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return wider
		}
	}

	for {
		if _, over := judge(block{text: kept, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return kept
		}
		next, ok := cutLastThought(kept)
		if !ok {
			break
		}
		kept = next
	}
	if forced, ok := hardFit(b); ok {
		return forced
	}
	// Nothing shorter both fits and reads.
	return b.text
}

// hardFit drops words off the end until the block fits, wherever the sentence
// ends. It mangles prose that no honest cut reaches, so it runs last, after
// every cut that leaves a comment somebody can read.
func hardFit(b block) ([]string, bool) {
	marker, indent, ok := commentShape(b.text)
	if !ok {
		return nil, false
	}
	var body []string
	for _, line := range prose(b.text) {
		body = append(body, stripMarker(line))
	}
	words := strings.Fields(strings.Join(body, " "))
	for len(words) > 0 {
		out := reflow(strings.Join(words, " "), indent, marker, max(floorChars, b.codeChars))
		if _, over := judge(block{text: out, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return out, true
		}
		words = words[:len(words)-1]
	}
	return nil, false
}

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
// It reports false for a block whose shape it cannot read, and for a last
// paragraph with no interior sentence end: dropParagraph and dropSentence own
// those.
func dropTrailingSentence(text []string) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, false
	}
	paras := paragraphs(text)
	last := -1
	for i, para := range paras {
		if !para.blank {
			last = i
		}
	}
	if last < 0 {
		return text, false
	}

	body := strings.Join(paras[last].lines, " ")
	sentences := ste.Sentences(body)
	if len(sentences) < 2 {
		return text, false
	}
	kept := strings.TrimSpace(strings.Join(sentences[:len(sentences)-1], " "))
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
	t := strings.TrimRight(strings.TrimSpace(line), `)]}"'`+"`")
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
	for _, marker := range []string{"//", "#", "*"} {
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
