// sentences.go holds ste/sentence-length for the prose of a comment. It is
// its own check: comments/length weighs a comment against its code, and a
// short comment can still hold a sentence past the cap.
package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/trace"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// FixerSentence is the fixer that divides a long sentence in a comment.
const FixerSentence = "comments/sentence-length"

// FixSentenceInBlock is the Fix of a long sentence in a block comment that holds more than one paragraph or a tag line.
const FixSentenceInBlock = "Rewrite it by hand as shorter sentences. The block holds more than one paragraph, and a rewrap joins them."

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
	// Line and EndLine are the rows the sentence covers, counted from one.
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
		words := lineWords(lines, p)
		rewrappable := rewrappable(lines, p)
		at := 0
		for _, sentence := range sentences(p.prose) {
			start := strings.Index(p.prose[at:], sentence)
			if start < 0 {
				continue
			}
			start += at
			at = start + len(sentence)
			for _, f := range ste.Check(sentence, 0) {
				if f.ID != ste.IDSentenceCap {
					continue
				}
				fix := f.Fix
				if !ste.ByHand(fix) && !rewrappable {
					fix = FixSentenceInBlock
				}
				first := len(strings.Fields(p.prose[:start]))
				last := first + max(len(strings.Fields(sentence))-1, 0)
				hits = append(hits, SentenceHit{
					Line:     rowOfWord(p, words, first) + 1,
					EndLine:  rowOfWord(p, words, last) + 1,
					Sentence: sentence,
					Tell:     f.Rule,
					Fix:      fix,
				})
			}
		}
	}
	return hits
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

// rewrappable reports a paragraph a rewrap keeps whole. A block comment with a
// blank line or a tag line such as "@param" holds a layout a rewrap joins.
func rewrappable(lines []string, p para) bool {
	if !isBlock(lines[p.lines[0]][len(p.code):]) {
		return true
	}
	for n, i := range p.lines {
		prose := lineProse(lines[i], p, n)
		if strings.HasPrefix(prose, "@") || (prose == "" && n > 0 && n < len(p.lines)-1) {
			return false
		}
	}
	return true
}

// lineWords counts the prose words each row of a paragraph carries.
func lineWords(lines []string, p para) []int {
	out := make([]int, len(p.lines))
	for n, i := range p.lines {
		out[n] = len(strings.Fields(lineProse(lines[i], p, n)))
	}
	return out
}

// lineProse is the prose row n of a paragraph carries, with no marker and no closer.
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

// rowOfWord answers the source row that holds word w of the paragraph's prose.
func rowOfWord(p para, words []int, w int) int {
	seen := 0
	for n, count := range words {
		seen += count
		if w < seen {
			return p.lines[n]
		}
	}
	return p.lines[len(p.lines)-1]
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
		out.WriteString(ste.FixSelected(sentence, func(id string) bool { return id == ste.IDSentenceCap }))
		at = start + len(sentence)
	}
	out.WriteString(prose[at:])
	return out.String()
}

// sentenceEdits answers an edit per comment paragraph whose long sentences
// divide. The paragraph is laid out again at the width it had. A comment after
// code, and a comment in a workflow, goes back on a single row.
func sentenceEdits(filename, src string, oneRow bool) []edit.Edit {
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
		if !judged(lines, p) || !rewrappable(lines, p) {
			continue
		}
		divided := divideSentences(p.prose)
		if divided == p.prose {
			continue
		}
		width := p.width
		if oneRow || p.code != "" {
			width = 1 << 20
		}
		wrapped := wrap(divided, p.marker, p.cont, width)
		if p.trailer != "" && len(wrapped) > 0 {
			wrapped[len(wrapped)-1] = strings.TrimRight(wrapped[len(wrapped)-1], " ") + " " + p.trailer
		}
		col := len(p.code)
		if len(wrapped) > 0 {
			wrapped[0] = wrapped[0][col:]
		}
		edits = append(edits, edit.Rows(src, p.lines[0], p.lines[len(p.lines)-1], col, wrapped))
	}
	return edits
}
