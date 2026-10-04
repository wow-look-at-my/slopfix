// overlong.go finds a comment longer than the code it documents.
//
// A comment earns its place by stopping the next mistake. A single longer
// than the code it documents becomes an essay the reader pays for on every
// pass. The rule is a proxy rather than a judgement of content: length is measurable.
//
// It reads a real syntax tree, so a span is exact rather than guessed, and
// treeblocks.go serves every grammar. Nothing here names a language.
//
// The repair cuts from the end, because a comment leads with its point. The
// opening sentence is never cut: a block trimmed to nothing is a worse edit
// than a block left long.
package commentfix

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/rules"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/syntax"
	"github.com/wow-look-at-my/slopfix/table"
	"github.com/wow-look-at-my/slopfix/trace"
)

// IDLength names this rule, on a report and on the command line alike.
const IDLength = "comments/length"

// floorChars is the size a comment may always be, whatever it documents.
const floorChars = 120

// LengthHit is a comment block that outweighs its code.
type LengthHit struct {
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
	// header is true for the comment above the package declaration.
	header bool
}

// Check reports every comment block in src that outweighs its code.
func CheckLength(filename, src string) []LengthHit {
	defer trace.Phase("rule/comments-length")()
	var hits []LengthHit
	for _, b := range blocks(filename, src) {
		if b.header {
			continue
		}
		tell, over := judge(b)
		if !over {
			continue
		}
		fixed := repair(b)
		_, stillOver := judge(block{text: fixed, codeLines: b.codeLines, codeChars: b.codeChars})
		hits = append(hits, LengthHit{
			ID:         IDLength,
			Tell:       tell,
			Sentence:   opening(b.text),
			Line:       b.start + 1,
			Repairable: b.exact && !sameText(fixed, b.text) && !stillOver,
		})
	}
	return hits
}

// Fix cuts every over-long comment block back inside its budget, from the end,
// stopping before the opening sentence.
func FixLength(filename, src string) (string, bool) {
	f := fixer.Open(filename, src, fixer.Options{Kind: fixer.Source})
	fixer.Run(f, fixer.Named(FixerLength))
	return f.Text(), f.Text() != src
}

// cutNote is what the report carries a single time any length cut lands.
const cutNote = "trailing comment prose"

// repairLength cuts every over-long comment block in f back inside its budget.
func repairLength(f *fixer.File) {
	defer trace.Phase("repair/comments-length")()
	if len(f.ApplyComments(lengthEdits(f.Path, f.Text(), f.MaxCommentLines)).Applied) > 0 {
		f.RemovedOnce(cutNote)
	}
}

// lengthEdits answers an edit per block that outweighs its code, or that runs
// past maxLines. The volume cap has no repair of its own, so this cut serves it.
func lengthEdits(filename, src string, maxLines int) []edit.Edit {
	var edits []edit.Edit
	for _, b := range blocks(filename, src) {
		if b.header {
			if maxLines <= 0 || len(prose(b.text)) <= maxLines {
				continue
			}
			if kept := capLines(b, maxLines); !sameText(kept, b.text) {
				edits = append(edits, edit.Rows(src, b.start, b.end-1, 0, kept))
			}
			continue
		}
		if maxLines > 0 && b.codeLines > maxLines {
			b.codeLines = maxLines
		}
		if _, over := judge(b); !over {
			continue
		}
		// A guessed span is reported but never rewritten. Wrong by a line, it
		// deletes the wrong sentence, and nobody reviews what a hook applied.
		if !b.exact {
			continue
		}
		kept := repair(b)
		// A repair that keeps the line count still shortens the text, and the
		// character half of the rule is what it answers.
		if sameText(kept, b.text) {
			continue
		}
		edits = append(edits, edit.Rows(src, b.start, b.end-1, 0, kept))
	}
	return edits
}

// judge measures a block against its code and names every measure it failed.
// Lines catch an essay; characters catch a dense paragraph.
func judge(b block) (string, bool) {
	lines, chars := measure(prose(b.text))
	// The free text after a lint pragma is prose, so its characters count.
	_, pragmaChars := measure(pragmaProse(b.text))
	chars += pragmaChars
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

// repair fits a block to its budget, and cuts pragma free text only when the prose cut is not enough.
func repair(b block) []string {
	out := repairProse(b)
	if fitsCode(out, b) {
		return out
	}
	// The pragma stays. Its free text is the prose left to cut.
	if bare := withoutPragmaProse(b.text); !sameText(bare, b.text) {
		b.text = bare
		return repairProse(b)
	}
	return out
}

// repairProse rewrites a block's prose and puts its directive lines back verbatim.
//
// Every repair path rebuilds the block out of prose() alone, which drops the
// directives: the rewrite then REPLACED them. A lost //go:embed leaves the
// variable it filled empty, and the tests reading it pass on nothing.
func repairProse(b block) []string {
	lead, body, trail := splitDirectives(b.text)
	// A comment with no code under it has nothing to be measured against, so no
	// amount of cutting brings it inside a budget.
	if b.codeLines == 0 {
		return append(append([]string{}, lead...), trail...)
	}
	if out, ok := repairBlockComment(b); ok {
		return out
	}
	if len(lead) == 0 && len(trail) == 0 {
		return trim(b)
	}
	for len(body) > 0 && isBlankComment(body[len(body)-1]) {
		body = body[:len(body)-1]
	}
	if len(body) == 0 {
		return b.text
	}
	bodyBlock := block{start: b.start, end: b.end, codeLines: b.codeLines, codeChars: b.codeChars, text: body, exact: b.exact}
	// A /* */ body is cut as a block, so its closer survives the cut.
	if out, ok := repairBlockComment(bodyBlock); ok {
		return append(append(append([]string{}, lead...), out...), trail...)
	}
	kept := trim(bodyBlock)
	if len(trail) > 0 {
		kept = withSeparator(kept, trail)
		// Doc and separator cannot fit, so the doc goes and the directive stays.
		if !fitsCode(kept, b) {
			kept = nil
		}
	}
	out := make([]string, 0, len(lead)+len(kept)+len(trail))
	out = append(out, lead...)
	out = append(out, kept...)
	return append(out, trail...)
}

// withSeparator adds the bare marker line gofmt puts between a doc and the
// directive after it, so the repair is measured as gofmt will leave it.
func withSeparator(kept, trail []string) []string {
	indent := trail[0][:len(trail[0])-len(strings.TrimLeft(trail[0], " \t"))]
	return append(append([]string{}, kept...), indent+"//")
}

// trim cuts the block's trailing prose until it fits, keeping the opening.
// A paragraph goes before a line does, and the opening paragraph always survives.
func trim(b block) []string {
	kept := b.text

	// Tighten before cutting. A padded comment fits after its filler is gone and
	// it is reflowed, and keeping the whole thought beats losing the last of it.
	if tightened, _, did := tighten(kept); did {
		if _, over := judge(block{text: tightened, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return tightened
		}
		kept = tightened
	}

	// The words can fit where the wrap does not. Laying them out at the budget's
	// own width drops none.
	if wider, did := widen(kept, max(floorChars, b.codeChars)); did {
		if _, over := judge(block{text: wider, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return wider
		}
	}

	for {
		if _, over := judge(block{text: kept, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
			return kept
		}
		if wider, did := widen(kept, max(floorChars, b.codeChars)); did {
			if _, over := judge(block{text: wider, codeLines: b.codeLines, codeChars: b.codeChars}); !over {
				return wider
			}
		}
		next, ok := cutLastThought(kept)
		if !ok {
			break
		}
		kept = next
	}
	// The STE opening sentence reads best, then a clause cut, then a word cut.
	opening, whole := steOpening(kept)
	var fits [][]string
	if whole && fitsCode(opening, b) {
		fits = append(fits, opening)
	}
	if clause, ok := clauseFit(b); ok {
		fits = append(fits, clause)
	}
	if words, ok := wordFit(block{text: kept, codeLines: b.codeLines, codeChars: b.codeChars}); ok {
		fits = append(fits, words)
	}
	if out, ok := preferred(fits); ok {
		return out
	}
	if whole {
		return opening
	}
	return kept
}

// steOpening answers a block's first sentence, repaired to STE and divided at a
// clause boundary when it runs past the cap. It reports false when no boundary
// brings the sentence under the cap.
func steOpening(text []string) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return nil, false
	}
	for _, para := range paragraphs(text) {
		if para.blank || para.verbatim {
			continue
		}
		sentences := ste.Sentences(ste.Fix(strings.Join(para.lines, " ")))
		if len(sentences) == 0 {
			return nil, false
		}
		first := strings.TrimSpace(sentences[0])
		if ste.WordCount(first) > ste.SentenceWordCap {
			// With no clause boundary the sentence stays whole, as ste/sentence-length leaves it.
			if clause, ok := ste.Leading(first); ok {
				first = strings.TrimSpace(ste.Fix(clause))
			}
		}
		if !endsSentence(first) {
			return nil, false
		}
		return reflow(first, indent, marker, wrapWidth), true
	}
	return nil, false
}

// sameText compares runs of lines by what they say. A line count cannot: a
// repair often keeps the count and still shortens the text.
func sameText(a, b []string) bool {
	return strings.Join(a, "\n") == strings.Join(b, "\n")
}

// clauseFit cuts the block back to the last clause boundary that fits, and
// closes what it keeps with a period. A whole clause still reads as a sentence.
func clauseFit(b block) ([]string, bool) {
	marker, indent, ok := commentShape(b.text)
	if !ok {
		return nil, false
	}
	var body []string
	for _, line := range leadingProse(b.text) {
		body = append(body, stripMarker(line))
	}
	text := strings.Join(strings.Fields(strings.Join(body, " ")), " ")
	for _, cut := range clauseCuts(text) {
		kept := strings.TrimRight(text[:cut], " ,;:-")
		words := strings.Fields(kept)
		if len(words) == 0 || !balanced(kept) || dangling.Contains(strings.ToLower(words[len(words)-1])) || !hasVerb(kept) {
			continue
		}
		if !endsSentence(kept) {
			kept += "."
		}
		if out, ok := fitReflow(kept, indent, marker, b); ok {
			return out, true
		}
	}
	return nil, false
}

// leadingProse answers the prose lines of a block up to its first code row.
func leadingProse(text []string) []string {
	lines := prose(text)
	for i, line := range lines {
		if codeRow(line) {
			return lines[:i]
		}
	}
	return lines
}

// hasVerb reports text the sentence parser finds a finite verb in.
func hasVerb(text string) bool {
	for _, c := range syntax.Parse(text, nil).Clauses {
		if c.Verb != nil {
			return true
		}
	}
	return false
}

// oneLine is a reflow width no comment reaches, so the prose stays on one line.
const oneLine = 1 << 20

// fitReflow lays text out at the narrowest width that fits the budget of b.
// The budget counts characters, not columns, so a single line is the last try.
func fitReflow(text, indent, marker string, b block) ([]string, bool) {
	budget := max(floorChars, b.codeChars)
	var out []string
	for _, width := range []int{min(budget, wrapWidth), budget, oneLine} {
		out = reflow(text, indent, marker, width)
		if fitsCode(out, b) {
			return out, true
		}
	}
	return out, false
}

// wordFit keeps the longest run of leading words that fits the budget, when
// no sentence or clause cut does. The run never ends on a dangling word and
// never splits a parenthesis, a quotation or a code span.
func wordFit(b block) ([]string, bool) {
	marker, indent, ok := commentShape(b.text)
	if !ok {
		return nil, false
	}
	var body []string
	for _, line := range leadingProse(b.text) {
		body = append(body, stripMarker(line))
	}
	render := func(text string) []string {
		out, _ := fitReflow(text, indent, marker, b)
		return out
	}
	return wordCut(strings.Join(body, " "), render, b)
}

// preferred answers the earliest cut that keeps at least a third of the words
// the longest cut keeps. A cut that reads well and keeps almost nothing loses.
func preferred(cuts [][]string) ([]string, bool) {
	most := 0
	for _, cut := range cuts {
		most = max(most, wordCount(cut))
	}
	for _, cut := range cuts {
		if 3*wordCount(cut) >= most {
			return cut, true
		}
	}
	return nil, false
}

// wordCount counts the words of a block's prose.
func wordCount(text []string) int {
	n := 0
	for _, line := range prose(text) {
		n += len(strings.Fields(stripMarker(line)))
	}
	return n
}

// fitsCode reports whether text fits the budget of the code under b.
func fitsCode(text []string, b block) bool {
	_, over := judge(block{text: text, codeLines: b.codeLines, codeChars: b.codeChars})
	return !over
}

// unwrapAside drops a parenthesis that encloses the whole of text, because no
// cut inside it closes.
func unwrapAside(text string) string {
	text = strings.TrimSpace(text)
	inner, ok := strings.CutPrefix(text, "(")
	if !ok {
		return text
	}
	for _, close := range []string{".)", ")."} {
		if rest, found := strings.CutSuffix(inner, close); found && balanced(rest) {
			return rest + "."
		}
	}
	if rest, found := strings.CutSuffix(inner, ")"); found && balanced(rest) {
		return rest
	}
	return text
}

// phraseEnds reports a cut after words[n-1] that ends a phrase: a closing mark
// ends it, a dash or an aside follows it, or the next word opens a new phrase.
func phraseEnds(words []string, n int) bool {
	if strings.ContainsAny(words[n-1][len(words[n-1])-1:], ",;:)") {
		return true
	}
	if next := words[n]; next == "—" || next == "--" || next == "-" || strings.HasPrefix(next, "(") {
		return true
	}
	return phraseOpeners.Contains(strings.ToLower(strings.Trim(words[n], "(\"'`")))
}

// phraseOpeners open a phrase the words before them can end without.
var phraseOpeners = set.Of(tailsClass("phrase-opener")...)

// wordCut keeps the longest leading run of words that render fits under b.
// A cut where a phrase ends is tried before a cut at any word.
func wordCut(prose string, render func(string) []string, b block) ([]string, bool) {
	words := strings.Fields(unwrapAside(prose))
	var cuts [][]string
	for _, atPhrase := range []bool{true, false} {
		if out, ok := longestCut(words, atPhrase, render, b); ok {
			cuts = append(cuts, out)
		}
	}
	return preferred(cuts)
}

// longestCut keeps the longest leading run of words that render fits under b.
// With atPhrase it cuts only where a phrase ends.
func longestCut(words []string, atPhrase bool, render func(string) []string, b block) ([]string, bool) {
	for n := len(words) - 1; n > 0; n-- {
		if atPhrase && !phraseEnds(words, n) {
			continue
		}
		last := strings.TrimRight(words[n-1], ",;:-")
		if last == "" || dangling.Contains(strings.ToLower(last)) {
			continue
		}
		kept := strings.Join(append(append([]string{}, words[:n-1]...), last), " ")
		if !balanced(kept) {
			continue
		}
		if !endsSentence(kept) {
			kept += "."
		}
		if out := render(kept); fitsCode(out, b) {
			return out, true
		}
	}
	return nil, false
}

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
func clauseCuts(text string) []int {
	marks := boundaryMarks()
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
	}
	return cuts
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
		// A code block holds no sentence to drop, so the cut looks past it.
		if !para.blank && !para.verbatim {
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
