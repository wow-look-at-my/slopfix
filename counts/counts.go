// Package counts finds the one sentence shape a document must not carry: an
// inventory count, where the prose says how many of something the repository,
// the project or the page itself currently holds.
//
// A count is true only until somebody adds or removes an item, and nothing
// corrects it when they do. The reader keeps trusting a number that has quietly
// gone wrong. Deleting the number is the whole repair, and it needs no
// judgement: "there are three sections" becomes "there are sections", which
// stays true after the next commit.
//
// A count needs two halves to qualify, and the second half is what keeps this
// usable. The FRAME says the sentence is talking about what is here: a
// possessive, a having verb, or a pointer into the page. The QUANTITY is a
// cardinal governing a plural noun. Both together is an inventory. The quantity
// alone is ordinary technical prose, and reporting that fires on every page.
package counts

import (
	"regexp"
	"sort"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfmt/markdown"
)

// Hit is a count found in a document: the phrase, the line holding it, and the
// byte span the phrase occupies. The span is what lets the cardinal be cut out
// in place instead of the write being refused.
type Hit struct {
	Phrase string
	Line   string
	// LineNo is where the phrase sits in the source, counting from the top.
	LineNo int
	Start  int
	End    int
}

// numberWords are the cardinals spelled out. "One" is deliberately absent: in
// English prose it is overwhelmingly a pronoun ("the wrong one", "one of them")
// and matching it reports far more good writing than bad. A document that says
// "one plugin" is also a document a single edit makes wrong, so this is a
// known, deliberate gap rather than an oversight.
const numberWords = `two|three|four|five|six|seven|eight|nine|ten|` +
	`eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|` +
	`nineteen|twenty|thirty|forty|fifty|sixty|seventy|eighty|ninety|dozen`

// quantity is a cardinal governing a plural noun, with adjectives allowed
// between them. The digit case is guarded in Go rather than here, by looking at
// the character in front of the match: RE2 has no lookbehind, and spending the
// frame's own separating space on a negated class stops the frame from ever
// meeting the quantity.
const quantity = `(?:\d{1,4}|\b(?:` + numberWords + `))` +
	`\s+(?:[a-z][a-z-]*\s+){0,3}?[a-z][a-z-]{2,}s\b`

// possessiveFrame is a determiner claiming the things belong here: "this repo's
// 15 plugins", "the payload's four steps", "our three servers".
var possessiveFrame = regexp.MustCompile(
	`(?i)\b(?:this|these|our|the)\s+(?:[a-z][a-z-]*\s+){0,2}?[a-z][a-z-]*'s\s+(` + quantity + `)`)

// havingFrame is a verb asserting possession or extent: "it ships two hooks",
// "there are three sections", "the plugin registers 15 servers".
var havingFrame = regexp.MustCompile(
	`(?i)\b(?:has|have|had|holds?|ships?|carries|carry|contains?|covers?|` +
		`includes?|lists?|defines?|registers?|installs?|answers?|serves?|` +
		`provides?|exposes?|declares?|embeds?|bundles?|comprises?|spans?|` +
		`there\s+(?:are|were))\s+(?:only\s+|just\s+|exactly\s+|all\s+)?(` + quantity + `)`)

// deicticFrame points inside the document: "the four rules below", "the three
// steps above". The count is of what this page shows, so editing the page
// breaks it.
var deicticFrame = regexp.MustCompile(
	`(?i)\b(?:the|these|those)\s+(` + quantity + `)\s+(?:\S+\s+){0,2}?(?:below|above|here)\b`)

var frames = []*regexp.Regexp{possessiveFrame, havingFrame, deicticFrame}

// measureNouns end a quantity that measures rather than counts. A limit, a size
// and a duration stay true after somebody adds a plugin, so even inside a frame
// there is nothing to go stale.
var measureNouns = set.Of[string](
	"seconds", "minutes", "hours", "days", "weeks", "months", "years",
	"milliseconds", "microseconds", "nanoseconds", "ms", "ns",
	"bytes", "kilobytes", "megabytes", "gigabytes", "kbs", "mbs", "gbs",
	"lines", "chars", "characters", "words", "columns", "pixels", "px",
	"times", "attempts", "retries", "levels", "degrees", "percents",
	"spaces", "tabs", "digits", "bits", "requests", "tokens",
)

// gapStopWords are the function words that prove the noun after them is not
// what the number counts. Without this check "it has 2 of the format drops"
// reads as a count of "drops", because a bare adjective run happily swallows
// "of the format".
var gapStopWords = set.Of[string](
	"of", "the", "a", "an", "in", "on", "to", "for", "and", "or", "is", "are",
	"was", "were", "that", "this", "with", "from", "by", "at", "as", "but",
	"if", "so", "than", "then", "when", "while", "not", "no", "it", "its",
)

var inlineCode = regexp.MustCompile("`[^`]*`")

// Check returns every inventory count stated in a document's own voice.
//
// The exemptions come from markdown.Split, which is the same splitter the wrap
// repair and the STE rules read. A fence, a table, a heading and an indented
// code block are all data. Inline backtick spans are blanked within a line: a
// number inside verbatim machinery is a literal, not the page's claim about
// itself, and it is also how this rule gets documented.
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

// cardinal matches the number this package cuts out: the digits or the spelled
// word at the head of a quantity, plus the space separating it from its noun.
var cardinal = regexp.MustCompile(`(?i)^(?:\d{1,4}|` + numberWords + `)\s+`)

// Strip removes the cardinal from every inventory count and returns the
// repaired text with the hits it acted on.
//
// Hits are cut back to front, so an earlier span's offsets stay valid.
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

// proseLine is one line of the document's own voice, with where it begins.
type proseLine struct {
	text   string
	no     int
	offset int
}

// proseLines returns the lines markdown.Split marks as prose, each with its
// byte offset, so a phrase found on one can be cut out of the document itself.
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

// continuesANumber reports that the character in front of a match makes it the
// tail of a longer number, so "pre-2.1.205 clients" is not read as a count.
func continuesANumber(text string, start int) bool {
	if start == 0 {
		return false
	}
	c := text[start-1]
	return c == '.' || (c >= '0' && c <= '9')
}

// isInventory rejects a quantity whose noun measures, and one reached through a
// function word.
func isInventory(phrase string) bool {
	words := strings.Fields(strings.ToLower(phrase))
	if len(words) < 2 {
		return false
	}
	if measureNouns.Contains(words[len(words)-1]) {
		return false
	}
	for _, w := range words[1 : len(words)-1] {
		if gapStopWords.Contains(w) {
			return false
		}
	}
	return true
}

// blankInlineCode replaces the contents of each backtick span with spaces,
// keeping every byte offset so the reported line still reads correctly.
func blankInlineCode(line string) string {
	return inlineCode.ReplaceAllStringFunc(line, func(s string) string {
		return strings.Repeat(" ", len(s))
	})
}
