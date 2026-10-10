// sentences.go holds ste/sentence-length for the prose of a comment. It is
// its own check: comments/length weighs a comment against its code, and a
// short comment can still hold a sentence past the cap.
package commentfix

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/trace"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// FixerSentence is the fixer that divides a long sentence in a comment.
const FixerSentence = "comments/sentence-length"

func init() {
	fixer.Register(fixer.Spec{
		Label:    FixerSentence,
		Families: []string{"ste"},
		Rules:    []string{ste.IDSentenceCap},
		Files:    []fixer.Kind{fixer.Source, fixer.Workflow},
		Place:    35,
		Repair: func(f *fixer.File) {
			f.ApplyComments(sentenceEdits(f.Path, f.Text(), f.Kind == fixer.Workflow))
		},
	})
}

// SentenceHit is a sentence in a comment over the STE word cap.
type SentenceHit struct {
	// Line and EndLine are the lines the sentence covers, counted from one.
	Line, EndLine int
	// Sentence quotes the sentence.
	Sentence string
	// Tell says the cap and the count, as the document rule says them.
	Tell string
	// Fix is the finding's remedy. ste.ByHand reports one no repair writes.
	Fix string
}

// CheckSentences reports every sentence in a comment of src over the word cap.
func CheckSentences(filename, src string) []SentenceHit {
	defer trace.Phase("rule/comments-sentence-length")()
	if IsGenerated(filename, src) {
		return nil
	}
	runs := treecomments.Runs(filename, src)
	if len(runs) == 0 {
		return nil
	}
	lines := strings.Split(src, "\n")
	var hits []SentenceHit
	for _, p := range paragraphsOf(lines, runs) {
		if !judged(lines, p) {
			continue
		}
		for _, seg := range segmentsOf(lines, p) {
			hits = append(hits, segmentHits(lines, p, seg)...)
		}
	}
	return hits
}

// segmentHits reports each sentence of a segment over the word cap.
func segmentHits(lines []string, p para, seg segment) []SentenceHit {
	words := segmentWords(lines, p, seg)
	var hits []SentenceHit
	at := 0
	for _, sentence := range sentences(seg.prose) {
		start := strings.Index(seg.prose[at:], sentence)
		if start < 0 {
			continue
		}
		start += at
		at = start + len(sentence)
		for _, f := range ste.Check(sentence, 0) {
			if f.ID != ste.IDSentenceCap {
				continue
			}
			first := len(strings.Fields(seg.prose[:start]))
			last := first + max(len(strings.Fields(sentence))-1, 0)
			hits = append(hits, SentenceHit{
				Line:     p.lines[seg.lines[lineOfWord(words, first)]] + 1,
				EndLine:  p.lines[seg.lines[lineOfWord(words, last)]] + 1,
				Sentence: sentence,
				Tell:     f.Rule,
				Fix:      f.Fix,
			})
		}
	}
	return hits
}

// segment is a run of lines of a comment paragraph that holds a single
// paragraph of prose. A blank line and a tag line such as "@param" end one.
type segment struct {
	// lines index the paragraph's lines, in order.
	lines []int
	// prose is the segment's prose, joined.
	prose string
}

// segmentsOf divides a paragraph into its segments.
func segmentsOf(lines []string, p para) []segment {
	var out []segment
	var current *segment
	prev := ""
	for n, i := range p.lines {
		prose := lineProse(lines[i], p, n)
		if prose == "" {
			current = nil
			continue
		}
		if current == nil || strings.HasPrefix(prose, "@") || opensListItem(prose) || lineEndsThought(prev, prose) {
			out = append(out, segment{})
			current = &out[len(out)-1]
		}
		current.lines = append(current.lines, n)
		current.prose = strings.TrimSpace(current.prose + " " + prose)
		prev = prose
	}
	return out
}

// opensListItem reports a line that opens a list item: "- ", "* ", "+ ", or a
// number with "." or ")" and a space.
func opensListItem(prose string) bool {
	for _, marker := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(prose, marker) {
			return true
		}
	}
	digits := len(prose) - len(strings.TrimLeft(prose, "0123456789"))
	rest := prose[digits:]
	return digits > 0 && (strings.HasPrefix(rest, ". ") || strings.HasPrefix(rest, ") "))
}

// lineEndsThought reports a line break that ends a thought with no stop. The
// line before ends on no mark and no word that opens a phrase. The line after
// opens on a capital or a list marker. A terse comment writes a sentence
// a line, as in "the byte at the checkpoint" and then "This handles a list".
func lineEndsThought(prev, next string) bool {
	prev = strings.TrimSpace(prev)
	if prev == "" || strings.ContainsAny(prev[len(prev)-1:], ".!?,;:-(—–/\\&|+=") {
		return false
	}
	fields := strings.Fields(prev)
	if dangling.Contains(strings.ToLower(trimWord(fields[len(fields)-1]))) {
		return false
	}
	first := []rune(next)[0]
	return unicode.IsUpper(first) || strings.HasPrefix(next, "- ") || strings.HasPrefix(next, "* ")
}

// judged reports a paragraph the rule reads: prose a person wrote for a reader.
// A license notice and a lint pragma are text a tool or a lawyer owns.
func judged(lines []string, p para) bool {
	if p.verbatim || p.prose == "" {
		return false
	}
	var text []string
	for _, i := range p.lines {
		if isLintPragma(lines[i]) {
			return false
		}
		text = append(text, lines[i])
	}
	return !isLicenseNotice(text)
}

// segmentWords counts the prose words each line of a segment carries.
func segmentWords(lines []string, p para, seg segment) []int {
	out := make([]int, len(seg.lines))
	for k, n := range seg.lines {
		out[k] = len(strings.Fields(lineProse(lines[p.lines[n]], p, n)))
	}
	return out
}

// lineProse is the prose line n of a paragraph carries, with no marker and no closer.
func lineProse(line string, p para, n int) string {
	if n == 0 {
		line = line[min(len(p.code), len(line)):]
	}
	_, prose, _, ok := splitBlock(line)
	if !ok {
		prose = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "*/"))
	}
	return prose
}

// lineOfWord answers which of the counted lines holds word w.
func lineOfWord(words []int, w int) int {
	seen := 0
	for k, count := range words {
		seen += count
		if w < seen {
			return k
		}
	}
	return len(words) - 1
}

// divideSentences divides each sentence of prose over the cap where both
// halves stay grammatical, as ste/sentence-length does in a document.
func divideSentences(prose string) string {
	var out strings.Builder
	at := 0
	for _, sentence := range sentences(prose) {
		start := strings.Index(prose[at:], sentence)
		if start < 0 {
			continue
		}
		start += at
		out.WriteString(prose[at:start])
		if ste.WordCount(ste.Masked(sentence)) <= ste.SentenceWordCap {
			out.WriteString(sentence)
		} else {
			// A semicolon and a splice are where a long sentence divides best, so the division repairs them too.
			out.WriteString(ste.FixSelected(sentence, func(id string) bool {
				return id == ste.IDSentenceCap || id == ste.IDSemicolon || id == ste.IDCommaSplice
			}))
		}
		at = start + len(sentence)
	}
	out.WriteString(prose[at:])
	return out.String()
}

// sentenceEdits answers an edit per comment paragraph whose long sentences
// divide. Each segment is laid out again at the width it had, with the marker
// and the indentation its lines had. A comment after code, and a comment in a
// workflow, keeps each segment on a single line.
func sentenceEdits(filename, src string, singleLine bool) []edit.Edit {
	if IsGenerated(filename, src) {
		return nil
	}
	runs := treecomments.Runs(filename, src)
	if len(runs) == 0 {
		return nil
	}
	lines := strings.Split(src, "\n")
	var edits []edit.Edit
	for _, p := range paragraphsOf(lines, runs) {
		if !judged(lines, p) {
			continue
		}
		out, changed := rewriteSegments(lines, p, singleLine || p.code != "")
		if !changed {
			continue
		}
		col := len(p.code)
		out[0] = out[0][min(col, len(out[0])):]
		edits = append(edits, edit.Lines(src, p.lines[0], p.lines[len(p.lines)-1], col, out))
	}
	return edits
}

// rewriteSegments answers the paragraph's lines with each long sentence divided.
// A line that holds no prose comes back as written.
func rewriteSegments(lines []string, p para, singleLine bool) ([]string, bool) {
	rewritten := make([][]string, len(p.lines))
	for n, i := range p.lines {
		rewritten[n] = []string{lines[i]}
	}
	changed := false
	for _, seg := range segmentsOf(lines, p) {
		divided := divideSentences(seg.prose)
		if divided == seg.prose {
			continue
		}
		changed = true
		firstN, lastN := seg.lines[0], seg.lines[len(seg.lines)-1]
		firstText, lastText := lines[p.lines[firstN]], lines[p.lines[lastN]]
		lead := prefixOf(firstText, lineProse(firstText, p, firstN))
		cont := p.cont
		if strings.Contains(lead, "/*") && strings.Contains(cont, "/*") {
			// A block's lines after its opener align under the opener's prose.
			indent := lead[:len(lead)-len(strings.TrimLeft(lead, " \t"))]
			cont = indent + strings.Repeat(" ", len(lead)-len(indent))
		}
		if len(seg.lines) > 1 {
			second := lines[p.lines[seg.lines[1]]]
			cont = prefixOf(second, lineProse(second, p, seg.lines[1]))
		}
		width := 0
		for _, n := range seg.lines {
			width = max(width, len(lines[p.lines[n]]))
		}
		if singleLine {
			width = 1 << 20
		}
		wrapped := wrap(divided, strings.TrimRight(lead, " ")+" ", strings.TrimRight(cont, " ")+" ", width)
		if closer := suffixOf(lastText, lineProse(lastText, p, lastN)); closer != "" {
			wrapped[len(wrapped)-1] = strings.TrimRight(wrapped[len(wrapped)-1], " ") + " " + closer
		}
		rewritten[firstN] = wrapped
		for _, n := range seg.lines[1:] {
			rewritten[n] = nil
		}
	}
	var out []string
	for _, r := range rewritten {
		out = append(out, r...)
	}
	return out, changed
}

// prefixOf answers what a line holds before its prose: indentation, marker and blank.
func prefixOf(line, prose string) string {
	if at := strings.Index(line, prose); at >= 0 && prose != "" {
		return line[:at]
	}
	return line
}

// suffixOf answers what a line holds after its prose, such as a block closer.
func suffixOf(line, prose string) string {
	if at := strings.LastIndex(line, prose); at >= 0 && prose != "" {
		return strings.TrimSpace(line[at+len(prose):])
	}
	return ""
}
