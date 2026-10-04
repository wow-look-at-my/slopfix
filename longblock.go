package slopfix

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/ste"
	"github.com/wow-look-at-my/slopfix/trace"
)

// IDLongBlock is a paragraph or a list item too long to read as one.
const IDLongBlock = "wrap/long-block"

// LongBlockCap is the most characters a prose block may hold.
const LongBlockCap = 1500

// LongBlockTarget is the size a division brings each part down to. The gap under LongBlockCap leaves room for the next edit.
const LongBlockTarget = LongBlockCap * 2 / 3

// LongBlockWordCap is the most words a part of a division may hold. It is the org's paragraph cap in words.
const LongBlockWordCap = 120

// LongBlockWordTarget is the word count a division brings each part down to.
const LongBlockWordTarget = LongBlockWordCap * 2 / 3

// longBlocks reports each prose block over LongBlockCap that a division can repair.
func longBlocks(content string) []ste.Finding {
	var out []ste.Finding
	for _, block := range markdown.Split(content) {
		if block.Kind != markdown.Prose {
			continue
		}
		text := block.Text()
		n := utf8.RuneCountInString(text)
		if n <= LongBlockCap || len(divide(text)) < 2 {
			continue
		}
		out = append(out, ste.Finding{
			Line:   block.Start,
			ID:     IDLongBlock,
			Rule:   fmt.Sprintf("over the %d-character cap for a paragraph or a list item at %d characters", LongBlockCap, n),
			Detail: string([]rune(text)[:60]) + "...",
			Fix:    "Divide it into paragraphs at sentence ends. `slopfix fix` does this.",
		})
	}
	return out
}

// longBlockEdits divides each long prose block into paragraphs. A list item
// keeps its marker on the first part, and indents the rest to its content.
func longBlockEdits(content string) []edit.Edit {
	var out []edit.Edit
	for _, block := range markdown.Split(content) {
		if block.Kind != markdown.Prose || utf8.RuneCountInString(block.Text()) <= LongBlockCap {
			continue
		}
		parts := divide(block.Text())
		if len(parts) < 2 {
			continue
		}
		first, rest := block.Indent, block.Indent
		if block.Marker != "" {
			first += block.Marker + " "
			rest += strings.Repeat(" ", len(block.Marker)+1)
		}
		lines := []string{first + parts[0]}
		for _, part := range parts[1:] {
			lines = append(lines, "", rest+part)
		}
		out = append(out, markdown.BlockEdit(content, block, lines))
	}
	return out
}

// Cut classes, best first.
const (
	cutSentence      = iota // a sentence end outside every bracket
	cutInnerSentence        // a sentence end inside a parenthesis
	cutWord                 // a blank between words
)

// cut is a place a paragraph may divide: the byte where the next part opens.
type cut struct{ at, class int }

// blockOpener is text that, at the head of a line, opens a list, a heading, a quotation, a fence, HTML or a table.
var blockOpener = regexp.MustCompile("^(?:[-*+]\\s|\\d{1,9}[.)]\\s|#{1,6}\\s|>|```|~~~|<|\\|)")

// cuts answers each place text may divide, in order. A code span, a link,
// bold text and a quotation never divide.
func cuts(text string) []cut {
	sentenceStarts := set.New[int]()
	from := 0
	for _, s := range ste.Sentences(text) {
		at := strings.Index(text[from:], s)
		if at < 0 {
			continue
		}
		sentenceStarts.Add(from + at)
		from += at + len(s)
	}
	var out []cut
	code, quote, bold := false, false, false
	bracket, paren := 0, 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '`':
			code = !code
		case code:
			continue
		case c == '*' && i+1 < len(text) && text[i+1] == '*':
			bold = !bold
			i++
		case c == '"':
			quote = !quote
		case c == '[':
			bracket++
		case c == ']' && bracket > 0:
			bracket--
		case c == '(':
			paren++
		case c == ')' && paren > 0:
			paren--
		case c == ' ' && i > 0 && i+1 < len(text) && text[i+1] != ' ':
			at := i + 1
			if quote || bold || bracket > 0 || blockOpener.MatchString(text[at:]) {
				continue
			}
			class := cutWord
			if sentenceStarts.Contains(at) {
				class = cutInnerSentence
				if paren == 0 {
					class = cutSentence
				}
			}
			out = append(out, cut{at, class})
		}
	}
	return out
}

// fits answers whether chunk holds no more than chars characters and words words.
func fits(chunk string, chars, words int) bool {
	return utf8.RuneCountInString(chunk) <= chars && len(strings.Fields(chunk)) <= words
}

// divide breaks text into parts of LongBlockTarget characters and
// LongBlockWordTarget words or less where it can. Each part ends at the best
// cut that fits: a sentence end first, then a sentence end inside a
// parenthesis, then a blank between words. A part also stops at the STE cap on
// the sentences in a paragraph.
func divide(text string) []string {
	defer trace.Phase("rule/long-block")()
	all := cuts(text)
	var parts []string
	start := 0
	for !fits(text[start:], LongBlockTarget, LongBlockWordTarget) {
		best := -1
		for i, c := range all {
			if c.at <= start {
				continue
			}
			chunk := text[start:c.at]
			if !fits(chunk, LongBlockTarget, LongBlockWordTarget) || len(ste.Sentences(chunk)) > ste.ParagraphSentenceCap {
				break
			}
			if best < 0 || c.class <= all[best].class {
				best = i
			}
		}
		if best < 0 || all[best].class == cutWord {
			// A sentence end past the target, but under the cap, reads better than a cut between words.
			for i, c := range all {
				if c.at > start && c.class < cutWord && fits(text[start:c.at], LongBlockCap, LongBlockWordCap) {
					best = i
					break
				}
			}
		}
		if best < 0 {
			for i, c := range all {
				if c.at > start {
					best = i
					break
				}
			}
		}
		if best < 0 {
			break
		}
		parts = append(parts, strings.TrimSpace(text[start:all[best].at]))
		start = all[best].at
	}
	return append(parts, strings.TrimSpace(text[start:]))
}

// The division runs after the join and the word repair, so it reads each block whole.
func init() {
	fixer.Register(fixer.Spec{
		Label:    "wrap/long-block",
		Families: []string{string(RuleWrap)},
		Rules:    []string{IDLongBlock},
		Files:    []fixer.Kind{fixer.Document},
		Place:    35,
		Repair: func(f *fixer.File) {
			f.Apply(longBlockEdits(f.Text()))
		},
	})
}
