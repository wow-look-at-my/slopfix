// Package counts finds an inventory count: prose saying how many of something
// the repository, the project or the page itself holds.
//
// Such a count is true until somebody adds or removes an item, and nothing
// corrects it when they do. Deleting the cardinal is the whole repair, and it
// needs no judgement. The sentence stays true through the next commit.
//
// This package is the document substrate of a rule the comment substrate
// shares. Which numbers count, and how much a sentence has to claim before a
// number does, live in cardinal. What is here is the document: which lines
// carry the page's own voice, and how a cardinal is cut out of a line.
package counts

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/markdown"
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

var inlineCode = regexp.MustCompile("`[^`]*`")

// Check returns every inventory count stated in a document's own voice.
func Check(content string) []Hit {
	return find(content, cardinal.Prose)
}

// Gate returns every count the merge gate's stale-count rule reports, which is the same walk over a substrate that asks
// for no frame around
func Gate(content string) []Hit {
	return find(content, cardinal.Gate)
}

func find(content string, substrate cardinal.Substrate) []Hit {
	var hits []Hit
	for _, line := range proseLines(content) {
		text := blankInlineCode(line.text)
		for _, found := range cardinal.Find(text, substrate) {
			hits = append(hits, Hit{
				Phrase: found.Text,
				Line:   strings.TrimSpace(line.text),
				LineNo: line.no,
				Start:  line.offset + found.Offset,
				End:    line.offset + found.Offset + len(found.Text),
			})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Start < hits[j].Start })
	return hits
}

func Strip(content string) (string, []Hit) {
	return strip(content, Check(content))
}

// StripGate is Strip over what the merge gate's stale-count rule reports.
func StripGate(content string) (string, []Hit) {
	return strip(content, Gate(content))
}

func strip(content string, hits []Hit) (string, []Hit) {
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
		number := cardinal.Leading.FindString(out[hit.Start:hit.End])
		if number == "" || measures(hit.Phrase) {
			continue
		}
		out = out[:hit.Start] + keepCapital(number, out[hit.Start+len(number):])
		cut = append(cut, hit)
	}
	sort.SliceStable(cut, func(i, j int) bool { return cut[i].Start < cut[j].Start })
	return out, cut
}

// measures reports whether a quantity is a measurement. It is still reported,
// but cutting its number leaves "every minutes", which states nothing.
func measures(phrase string) bool {
	fields := strings.Fields(phrase)
	return len(fields) > 0 && cardinal.IsUnit(fields[len(fields)-1])
}

// keepCapital moves the capital of a cut cardinal onto the word after it.
func keepCapital(number, rest string) string {
	first, _ := utf8.DecodeRuneInString(number)
	if !unicode.IsUpper(first) {
		return rest
	}
	next, width := utf8.DecodeRuneInString(rest)
	return string(unicode.ToUpper(next)) + rest[width:]
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

// blankInlineCode replaces each backtick span with spaces, keeping every byte
// offset so the reported line still reads correctly.
func blankInlineCode(line string) string {
	return inlineCode.ReplaceAllStringFunc(line, func(s string) string {
		return strings.Repeat(" ", len(s))
	})
}
