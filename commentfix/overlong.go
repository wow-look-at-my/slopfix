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
	"slices"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/memo"
	"github.com/wow-look-at-my/slopfix/ste"
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

// Check reports every comment block in src that outweighs its code and that
// the repair brings inside its budget. A block no cut fits keeps its opening
// sentence whole, so no repair answers it and no finding names it.
func CheckLength(filename, src string) []LengthHit {
	defer trace.Phase("rule/comments-length")()
	return slices.Clone(lengthHits.Get(fileKey{filename, src}, func() []LengthHit { return checkLength(filename, src) }))
}

// lengthHits keeps CheckLength's answer for the rest of the run.
var lengthHits = memo.New[fileKey, []LengthHit](512)

func checkLength(filename, src string) []LengthHit {
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
		if _, stillOver := judge(block{text: fixed, codeLines: b.codeLines, codeChars: b.codeChars}); stillOver || !b.exact || sameText(fixed, b.text) {
			continue
		}
		hits = append(hits, LengthHit{
			ID:         IDLength,
			Tell:       tell,
			Sentence:   opening(b.text),
			Line:       b.start + 1,
			Repairable: true,
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
				edits = append(edits, edit.Lines(src, b.start, b.end-1, 0, kept))
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
		edits = append(edits, edit.Lines(src, b.start, b.end-1, 0, kept))
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
	// A single sentence under the STE cap is the least a comment can say, so its characters are never an essay.
	if chars > limit && !oneSentence(b.text) {
		tells = append(tells, "the comment runs longer than the code it documents")
	}
	if len(tells) == 0 {
		return "", false
	}
	return strings.Join(tells, ", and "), true
}

// oneSentence reports a block whose prose is a single sentence under the STE
// word cap. A line with no letter is a banner, not prose.
func oneSentence(text []string) bool {
	var words []string
	for _, line := range prose(text) {
		body := stripMarker(line)
		if strings.IndexFunc(body, unicode.IsLetter) < 0 {
			continue
		}
		words = append(words, strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(body), "*/")))
	}
	joined := strings.TrimSpace(strings.Join(words, " "))
	if joined == "" {
		return false
	}
	found := ste.Sentences(joined)
	return len(found) == 1 && ste.WordCount(found[0]) <= ste.SentenceWordCap && !hasBanner(text)
}

// hasBanner reports a comment line whose prose holds no letter, such as a
// rule of "=" signs. A banner has no reader, so a block that holds one is weighed whole.
func hasBanner(text []string) bool {
	for _, line := range prose(text) {
		body := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(stripMarker(line)), "*/"))
		if body != "" && strings.IndexFunc(body, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) < 0 {
			return true
		}
	}
	return false
}

// withoutBanners drops each banner line of a block, and the blank comment lines
// left at either end.
func withoutBanners(text []string) []string {
	if !hasBanner(text) {
		return text
	}
	var out []string
	for _, line := range text {
		body := strings.TrimSpace(stripMarker(line))
		if body != "" && strings.IndexFunc(body, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) < 0 && !isDirectiveLine(line) {
			continue
		}
		out = append(out, line)
	}
	for len(out) > 0 && isBlankComment(out[0]) {
		out = out[1:]
	}
	for len(out) > 0 && isBlankComment(out[len(out)-1]) {
		out = out[:len(out)-1]
	}
	return out
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

// repair answers repairUncached once for each block.
func repair(b block) []string {
	key := blockKey{text: strings.Join(b.text, "\n"), codeLines: b.codeLines, codeChars: b.codeChars, exact: b.exact, header: b.header}
	return slices.Clone(repaired.Get(key, func() []string { return repairUncached(b) }))
