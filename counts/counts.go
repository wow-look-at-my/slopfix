// Package counts finds an inventory count: prose saying how many of something
// the repository, the project or the page itself holds.
//
// Such a count is true until somebody adds or removes an item, and nothing
// corrects it when they do. Deleting the cardinal is the whole repair, and it
// needs no judgement. The sentence stays true through the next commit.
//
// A count qualifies only with both halves. The FRAME says the sentence talks
// about what is here: a possessive, a having verb, or a pointer into the page.
// The QUANTITY is a cardinal governing a plural noun. The quantity alone is
// ordinary technical prose, and reporting that fires on every page.
package counts

import (
	"regexp"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfmt/markdown"
)

// ID names this rule, on a report and on the command line alike.
const ID = "counts/inventory-count"

// Hit is a count found in a document. The byte span is what lets the cardinal
// be cut out in place instead of the write being refused.
type Hit struct {
	Phrase string
	Line   string
	// LineNo is where the phrase sits in the source, counting from the top.
	LineNo int
	Start  int
	End    int
}

// numberWords are the cardinals spelled out. The singular is deliberately
// absent: in English prose it is overwhelmingly a pronoun, and matching it
// reports far more good writing than bad. That is a known gap.
const numberWords = `two|three|four|five|six|seven|eight|nine|ten|` +
	`eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|` +
	`nineteen|twenty|thirty|forty|fifty|sixty|seventy|eighty|ninety|dozen`

// quantity is a cardinal governing a plural noun, adjectives allowed between.
// RE2 has no lookbehind, so continuesANumber guards a digit.
const quantity = `(?:\d{1,4}|\b(?:` + numberWords + `))` +
	`\s+(?:[a-z][a-z-]*\s+){0,3}?[a-z][a-z-]{2,}s\b`

// possessiveFrame is a determiner claiming the things belong here, as in "this
// repo's plugins" or "the payload's steps".
var possessiveFrame = regexp.MustCompile(
	`(?i)\b(?:this|these|our|the)\s+(?:[a-z][a-z-]*\s+){0,2}?[a-z][a-z-]*'s\s+(` + quantity + `)`)

// havingFrame is a verb asserting possession or extent, as in "it ships hooks"
// or "there are sections". A reporting verb belongs here too: a document that
// says what something MEASURES, TAKES or COSTS has written a reading down, and
// the reading moves.
var havingFrame = regexp.MustCompile(
	`(?i)\b(?:has|have|had|holds?|ships?|carries|carry|contains?|covers?|` +
		`includes?|lists?|defines?|registers?|installs?|answers?|serves?|` +
		`provides?|exposes?|declares?|embeds?|bundles?|comprises?|spans?|` +
		`measures?|measured|takes?|took|costs?|needs?|uses?|used|` +
		`runs?\s+(?:in|for)|completes?\s+in|finishes(?:\s+in)?|` +
		`there\s+(?:are|were))\s+(?:only\s+|just\s+|exactly\s+|all\s+|about\s+|roughly\s+|around\s+|under\s+|over\s+)?(` + quantity + `)`)

// deicticFrame points inside the document, as in "the rules below". The count
// is of what this page shows, so editing the page breaks it.
var deicticFrame = regexp.MustCompile(
	`(?i)\b(?:the|these|those)\s+(` + quantity + `)\s+(?:\S+\s+){0,2}?(?:below|above|here)\b`)

var frames = []*regexp.Regexp{possessiveFrame, havingFrame, deicticFrame}

// A measurement inside a frame is a count: a budget gets raised and a suite
// gets slower. It reads with more authority than a tally, because it looks
// like an instrument produced it. The number belongs where it is enforced.

// gapStopWords are function words proving the noun after them is not what the
// cardinal counts. A bare adjective run happily swallows "of the format".
var gapStopWords = set.Of[string](
	"of", "the", "a", "an", "in", "on", "to", "for", "and", "or", "is", "are",
	"was", "were", "that", "this", "with", "from", "by", "at", "as", "but",
	"if", "so", "than", "then", "when", "while", "not", "no", "it", "its",
)

var inlineCode = regexp.MustCompile("`[^`]*`")

// Check returns every inventory count stated in a document's own voice.
//
// The exemptions come from markdown.Split, the same splitter the wrap repair
// and the STE rules read. A backtick span is blanked within its line: a
// cardinal inside verbatim machinery is a literal, not the page's own claim,
// and quoting the shape is how this rule gets documented.
func Check(content string) []Hit {
	var hits []Hit
	for _, line := range proseLines(content) {
		text := blankInlineCode(line.text)
		seen := set.New[string]()
		for _, frame := range frames {
			for _, at := range frame.FindAllStringSubmatchIndex(text, -1) {
				start, end := at[2], at[3]
				phrase := text[start:end]
				if continuesANumber(text, start) || !isInventory(phrase) || seen.Contains(phrase) {
					continue
				}
				seen.Add(phrase)
				hits = append(hits, Hit{
					Phrase: phrase,
					Line:   strings.TrimSpace(line.text),
					LineNo: line.no,
					Start:  line.offset + start,
					End:    line.offset + end,
				})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Start < hits[j].Start })
	return hits
}

// cardinal matches what Strip cuts: a quantity's leading number and its space.
var cardinal = regexp.MustCompile(`(?i)^(?:\d{1,4}|` + numberWords + `)\s+`)

// Strip removes the cardinal from every inventory count and returns the
// repaired text with the hits it acted on. Cutting runs back to front, so an
// earlier span's offsets stay valid.
func Strip(content string) (string, []Hit) {
	hits := Check(content)
	if len(hits) == 0 {
		return content, nil
	}
	ordered := append([]Hit(nil), hits...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Start > ordered[j].Start })

	out := content
	var cut []Hit
	for _, hit := range ordered {
		if hit.Start < 0 || hit.End > len(out) {
			continue
		}
		number := cardinal.FindString(out[hit.Start:hit.End])
		if number == "" {
			continue
		}
		out = out[:hit.Start] + out[hit.Start+len(number):]
		cut = append(cut, hit)
	}
	sort.SliceStable(cut, func(i, j int) bool { return cut[i].Start < cut[j].Start })
	return out, cut
}

// proseLine is a line of the document's own voice, with where it begins.
type proseLine struct {
	text   string
	no     int
	offset int
}

// proseLines returns the lines markdown.Split marks as prose, each with its
// byte offset, so a phrase found there can be cut out of the document.
func proseLines(content string) []proseLine {
	lines := strings.Split(content, "\n")
	offsets := make([]int, len(lines))
	at := 0
	for i, line := range lines {
		offsets[i] = at
		at += len(line) + 1
	}
	var out []proseLine
	for _, block := range markdown.Split(content) {
		if block.Kind != markdown.Prose {
			continue
		}
		for n, text := range block.Lines {
			i := block.Start - 1 + n
			if i < 0 || i >= len(lines) {
				continue
			}
			out = append(out, proseLine{text: text, no: i + 1, offset: offsets[i]})
		}
	}
	return out
}

// continuesANumber reports a match that is the tail of a longer number, so a
// version string is not read as a count.
func continuesANumber(text string, start int) bool {
	if start == 0 {
		return false
	}
	c := text[start-1]
	return c == '.' || (c >= '0' && c <= '9')
}

// isInventory rejects a quantity reached through a function word. A quantity
// whose noun measures is an inventory like any other, per the note above.
func isInventory(phrase string) bool {
	words := strings.Fields(strings.ToLower(phrase))
	if len(words) < 2 {
		return false
	}
	for _, w := range words[1 : len(words)-1] {
		if gapStopWords.Contains(w) {
			return false
		}
	}
	return true
}

// blankInlineCode replaces each backtick span with spaces, keeping every byte
// offset so the reported line still reads correctly.
func blankInlineCode(line string) string {
	return inlineCode.ReplaceAllStringFunc(line, func(s string) string {
		return strings.Repeat(" ", len(s))
	})
}
