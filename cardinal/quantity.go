// quantity.go is the finder both document substrates read: a cardinal
// governing a plural noun. It also holds what each of them exempts.
//
// Both spell the shape with different tolerances, and the spellings sit here
// together rather than in the packages that read them. Neither is derived from
// the other. They are what both rules have always matched, and a fold that
// merged them would move verdicts on the merge gate.
package cardinal

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// proseQuantity is the inventory-count spelling: a plural cardinal, up
var proseQuantity = `(?:\d{1,4}|\b(?:` + proseAlt + `))` +
	`\s+(?:[a-z][a-z-]*\s+){0,3}?[a-z][a-z-]{2,}s\b`

// gateQuantity is the merge gate's spelling. It reads a shorter list of
// words, any run of digits, and a shorter adjective gap.
var gateQuantity = regexp.MustCompile(`(?i)\b(?:` + gateAlt + `|[0-9]+)` +
	`\s+(?:[0-9]+(?:\.[0-9]+)?\s+[a-z]+\s+)?(?:[a-z-]+\s+){0,2}?[a-z]+s\b`)

// Match is a quantity the pattern found, and the parts an exemption asks about.
type Match struct {
	// At is where the cardinal starts in the text.
	At int
	// Text is the whole quantity, the cardinal and the noun it governs.
	Text string
	// Noun is the plural noun the cardinal governs.
	Noun string
}

// Exemption reports whether a matched quantity counts nothing here.
type Exemption func(text string, q Match) bool

// quantities returns every quantity in the text, for a substrate that asks for
// no frame around it.
func quantities(text string, s Substrate) []Token {
	var out []Token
	for _, at := range s.quantity.FindAllStringIndex(text, -1) {
		q := quantityAt(text, at[0], at[1])
		if exemptQuantity(text, q, s.Exempt) {
			continue
		}
		out = append(out, Token{Offset: q.At, Text: q.Text})
	}
	return out
}

// quantityAt reads the parts of the span the pattern matched. The noun is the
// last word of it, because every spelling of the shape ends on the noun.
func quantityAt(text string, start, end int) Match {
	phrase := text[start:end]
	noun := ""
	if fields := strings.Fields(phrase); len(fields) > 0 {
		noun = strings.ToLower(fields[len(fields)-1])
	}
	return Match{At: start, Text: phrase, Noun: noun}
}

func exemptQuantity(text string, q Match, rules []Exemption) bool {
	for _, rule := range rules {
		if rule(text, q) {
			return true
		}
	}
	return false
}

// ContinuesANumber exempts a match that is the tail of a longer number, so a
// version string is not read as a count.
func ContinuesANumber(text string, q Match) bool {
	if q.At == 0 {
		return false
	}
	c := text[q.At-1]
	return c == '.' || (c >= '0' && c <= '9')
}

// FunctionWordGap exempts a quantity reached through a function word. A bare
// adjective run happily swallows "of the format".
func FunctionWordGap(_ string, q Match) bool {
	words := strings.Fields(strings.ToLower(q.Text))
	if len(words) < 2 {
		return true
	}
	for _, w := range words[1 : len(words)-1] {
		if gapStopWords.Contains(w) {
			return true
		}
	}
	return false
}

// gapStopWords are function words proving the noun after them is not what the
// cardinal counts.
var gapStopWords = set.Of[string](
	"of", "the", "a", "an", "in", "on", "to", "for", "and", "or", "is", "are",
	"was", "were", "that", "this", "with", "from", "by", "at", "as", "but",
	"if", "so", "than", "then", "when", "while", "not", "no", "it", "its",
)

// AfterAnArticle exempts a number after "a" or "an". A tally takes no singular
// article, so the number there is itself the noun: "a 404 buries it".
func AfterAnArticle(text string, q Match) bool {
	before := strings.Fields(strings.ToLower(text[:q.At]))
	if len(before) == 0 {
		return false
	}
	last := before[len(before)-1]
	return last == "a" || last == "an"
}

// NotAPluralNoun exempts a match whose last word the tagger reads as something
// other than a plural noun. The pattern only asks for a trailing s, so "a
// buries it" and "the other confirms" match on a verb.
func NotAPluralNoun(text string, q Match) bool {
	end := q.At + len(q.Text)
	words := syntax.Parse(text, nil).Words
	for i, w := range words {
		if w.End != end {
			continue
		}
		if w.Tag == "VBZ" && countsANominal(words, q.At, i) {
			return false
		}
		return w.Tag != "NNS" && w.Tag != "NNPS"
	}
	return false
}

// nominalTags are the tags a modifier between a cardinal and its noun carries.
var nominalTags = set.Of("CD", "NN", "NNP", "JJ")

// auxiliaries are verbs whose -s form is never a plural noun.
var auxiliaries = set.Of("is", "has", "does", "was")

// countsANominal reports a cardinal at start followed only by modifiers up to
// the word at last, in a sentence whose verb came before it. The tagger reads
// "sends four PATCH requests" as a subject and a verb, and the earlier verb
// is what leaves the last word a noun. "128 KiB is" has no such verb.
func countsANominal(words []syntax.Word, start, last int) bool {
	if auxiliaries.Contains(words[last].Lower()) {
		return false
	}
	first := -1
	for i, w := range words[:last] {
		if w.Start == start {
			first = i
			break
		}
	}
	if first < 0 || last-first < 2 || words[first].Tag != "CD" {
		return false
	}
	for _, w := range words[first+1 : last] {
		if !nominalTags.Contains(w.Tag) {
			return false
		}
	}
	return verbBefore(words, first)
}

// verbBefore reports a verb earlier in the sentence than the word at i.
func verbBefore(words []syntax.Word, i int) bool {
	for j := i - 1; j >= 0 && words[j].Tag != "."; j-- {
		if strings.HasPrefix(words[j].Tag, "VB") || words[j].Tag == "MD" {
			return true
		}
	}
	return false
}

// ChoiceAmongASet exempts the size of a set something is picked from. "one of
// things" loses its meaning without the count, and the cut leaves "one of
// things".
func ChoiceAmongASet(text string, q Match) bool {
	before := strings.Fields(strings.ToLower(text[:q.At]))
	if len(before) < 2 || before[len(before)-1] != "of" {
		return false
	}
	return choosers.Contains(before[len(before)-2])
}

var choosers = set.Of[string]("one", "either", "neither", "any", "each", "none", "both", "all")

// leadingNumber is the cardinal a quantity opens with.
func leadingNumber(q Match) string {
	fields := strings.Fields(q.Text)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// bare strips the punctuation that sits against a word in a sentence.
func bare(word string) string {
	return strings.Trim(strings.ToLower(word), ".,;:()\"'`")
}

// joiners link values the way a list of status codes does.
var joiners = set.Of("or", "and", "nor")

// StatusCode exempts an HTTP status code: one that heads a reply noun ("the
// responses"), follows the word status, or sits in a list beside another code
// ("201 or 409", "200 and 404"). A status is a value, so no edit that adds an
// item moves it.
func StatusCode(text string, q Match) bool {
	if !InClass(leadingNumber(q), "status-code") {
		return false
	}
	if fields := strings.Fields(q.Text); len(fields) > 1 && InClass(fields[1], "status-noun") {
		return true
	}
	before := strings.Fields(text[:q.At])
	if n := len(before); n > 0 {
		last := bare(before[n-1])
		if last == "status" || InClass(last, "status-code") {
			return true
		}
		if n > 1 && joiners.Contains(last) && InClass(bare(before[n-2]), "status-code") {
			return true
		}
	}
	after := strings.Fields(text[q.At+len(leadingNumber(q)):])
	return len(after) > 1 && joiners.Contains(bare(after[0])) && InClass(bare(after[1]), "status-code")
}

// Labeled exempts a number that names an item rather than counting a set: one
// after a label word, as in "Migration 014" or "HTTP 404".
func Labeled(text string, q Match) bool {
	before := strings.Fields(text[:q.At])
	return len(before) > 0 && InClass(bare(before[len(before)-1]), "label")
}

// IssueNumber exempts the digits of the literal shape "issue #<digits>". An
// issue number never changes.
func IssueNumber(text string, q Match) bool {
	return issueRef.MatchString(text[:q.At])
}

var issueRef = regexp.MustCompile(`(?i)\bissue #$`)

// InExpression exempts a number that is arithmetic rather than a count. The
// digits in an expression or a range name no set of items.
func InExpression(text string, q Match) bool {
	if q.At == 0 {
		return false
	}
	before := []rune(text[:q.At])
	prev := before[len(before)-1]
	switch prev {
	case '-', '−', '+', '/', '*', '=', '.', ',', '_':
		return true
	}
	return unicode.IsDigit(prev) || inEquation(text, q.At)
}

// inEquation reports a number in a clause that holds an equals sign. An
// equation states an identity, such as a unit conversion, and no tally.
func inEquation(text string, at int) bool {
	start := strings.LastIndexAny(text[:at], "(;,") + 1
	end := len(text)
	if i := strings.IndexAny(text[at:], ");,"); i >= 0 {
		end = at + i
	}
	return strings.Contains(text[start:end], " = ")
}
