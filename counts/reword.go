package counts

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/edit"
)

// RewordAt is reword for a caller that holds the phrase but no Hit: the
// cardinal at start, and the words it governs.
func RewordAt(content string, start int, phrase string) (edit.Edit, bool) {
	return reword(content, Hit{Phrase: phrase, Start: start, End: start + len(phrase)})
}

// reword answers the edit that takes the cardinal out of a hit. The words in
// front of the number decide what goes in its place. A rate reads "every few",
// a cap reads "a bounded number of", and a unit takes a vague amount. After a
// preposition or a noun the number becomes "multiple". Anywhere else it goes.
func reword(content string, hit Hit) (edit.Edit, bool) {
	loc := cardinal.Leading.FindStringIndex(content[hit.Start:hit.End])
	if loc == nil {
		return edit.Edit{}, false
	}
	from, to := hit.Start, hit.Start+loc[1]
	number := strings.TrimSpace(content[from:to])
	if from > 0 && content[from-1] == '~' {
		from--
	}
	n := value(number)
	noun := strings.ToLower(lastField(hit.Phrase))
	unit := cardinal.IsUnit(noun)
	sign, signAt := boundSign(content, from)

	before := wordsBefore(content, from)
	last := func(k int) string {
		if len(before) < k {
			return ""
		}
		var parts []string
		for _, w := range before[len(before)-k:] {
			parts = append(parts, w.lower)
		}
		return strings.Join(parts, " ")
	}
	drop := func(k int) {
		from = before[len(before)-k].start
		before = before[:len(before)-k]
	}

	for len(before) > 0 && approximations.Contains(last(1)) {
		drop(1)
	}

	var text string
	switch {
	case n == 0 && !measures(hit.Phrase):
		// Zero of a thing is none of it, and "a couple of" would claim some.
		text = "no "
	case measures(hit.Phrase):
		to = hit.Start + loc[1] + len(unitAfter(content[hit.Start+loc[1]:]))
		if sign.text != "" {
			from = signAt
		}
	case sign.text != "":
		// "<=750 lines" caps the lines, so it reads as the words "at most" do.
		from = signAt
		text = sign.reword(n)
	case len(before) > 0 && prepositionFloors.Contains(last(1)):
		// "over" stays: "spreads over 32 banks" spans the banks.
		text = vague(n)
	case len(before) > 0 && rates.Contains(last(1)):
		drop(1)
		text = "every few "
	case ceilings.Contains(last(3)):
		drop(3)
		text = "a bounded number of "
	case ceilings.Contains(last(2)):
		drop(2)
		text = "a bounded number of "
	case len(before) > 0 && ceilings.Contains(last(1)):
		drop(1)
		text = "a bounded number of "
	case floorsAbove.Contains(last(2)):
		drop(2)
		text = vague(n + 1)
	case floors.Contains(last(3)):
		drop(3)
		text = vague(n)
	case floors.Contains(last(2)):
		drop(2)
		text = vague(n)
	case determined(before):
		text = ""
	case unit:
		text = vague(n)
	case len(before) > 0 && (cardinal.InClass(last(1), "preposition") || followsANoun(content, hit.Start)):
		text = "multiple "
	}

	e := edit.Edit{Start: from, End: to, Text: text, Cut: []string{hit.Phrase}}
	first, _ := utf8.DecodeRuneInString(content[from:])
	// An emphatic "FOUR" mid-sentence passes no capital to what follows it.
	capital := opensSentence(content, from) && (unicode.IsUpper(first) || !unicode.IsLetter(first))
	switch {
	case !capital:
	case text != "":
		e.Text = strings.ToUpper(text[:1]) + text[1:]
	default:
		next, width := utf8.DecodeRuneInString(content[e.End:])
		e.Text = string(unicode.ToUpper(next))
		e.End += width
	}
	return e, true
}

// measures reports a number whose next word is a unit ahead of the noun, as
// in "64 MiB chunks".
func measures(phrase string) bool {
	fields := strings.Fields(phrase)
	return len(fields) > 2 && cardinal.IsUnit(fields[1])
}

var leadingWord = regexp.MustCompile(`^[ \t]*\S+[ \t]+`)

// unitAfter is the unit word at the start of rest, with the blanks around it.
func unitAfter(rest string) string {
	return leadingWord.FindString(rest)
}

// vague says how many without a number. It keeps the scale, so "90 seconds"
// does not read as "a few seconds".
func vague(n int) string {
	switch {
	case n <= 2:
		return "a couple of "
	case n <= 5:
		return "a few "
	case n <= 20:
		return "several "
	}
	return "many "
}

// approximations hedge a number, and go with it.
var approximations = set.Of[string]("about", "roughly", "around", "approximately",
	"nearly", "almost", "only", "just", "exactly", "some", "precisely")

// rates make the number an interval, so it reads as "every few".
var rates = set.Of[string]("every", "each", "per")

// ceilings cap the number. The cap is the fact, so the repair keeps that much.
var ceilings = set.Of[string]("at most", "up to", "no more than", "fewer than",
	"less than", "not more than", "under", "below", "within", "max", "maximum")

// bound is a comparison sign written against a number, as in "<=750 lines".
type bound struct {
	text string
	// floor marks a sign that sets a minimum, and strict one that excludes the number itself.
	floor, strict bool
}

// reword says what a bound on n reads as once the number is gone. A cap
// keeps the cap, and a floor keeps the scale the way "at least" does.
func (b bound) reword(n int) string {
	switch {
	case !b.floor:
		return "a bounded number of "
	case b.strict:
		return vague(n + 1)
	}
	return vague(n)
}

var (
	lessOrEqual    = string(rune(0x2264))
	greaterOrEqual = string(rune(0x2265))
)

// boundSigns are the signs a bound writes, the longer spelling first so "<="
// is never read as "<".
var boundSigns = []bound{
	{text: "<="}, {text: lessOrEqual}, {text: "<", strict: true},
	{text: ">=", floor: true}, {text: greaterOrEqual, floor: true}, {text: ">", floor: true, strict: true},
}

// boundSign answers the sign that stands in front of the number at, with the
// blanks between them, and where the sign starts. A lone "<" or ">" is a bound
// only against the number and after no word, because a blockquote marker and
// the end of a tag spell it too.
func boundSign(content string, at int) (bound, int) {
	head := strings.TrimRight(content[:at], " \t")
	for _, b := range boundSigns {
		if !strings.HasSuffix(head, b.text) {
			continue
		}
		start := len(head) - len(b.text)
		if len(b.text) == 1 && (len(head) != at || (start > 0 && joinsASign(content[start-1]))) {
			continue
		}
		return b, start
	}
	return bound{}, at
}

// joinsASign reports a byte that makes a lone "<" or ">" part of something
// else: a word, as the end of a tag, or an arrow.
func joinsASign(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || strings.IndexByte("-=<>", c) >= 0
}

// floors set a minimum the vague word meets.
var floors = set.Of[string]("at least", "no fewer than", "no less than", "not less than")

// floorsAbove exclude the number itself, so the vague word reads one higher.
var floorsAbove = set.Of[string]("more than")

// prepositionFloors stay in the sentence, because each also names a span.
var prepositionFloors = set.Of[string]("over", "above")

// determined reports whether a determiner or a possessive governs the noun
// already, as in "the three rules" or "this repo's plugins". The number then
// goes alone.
func determined(before []word) bool {
	if len(before) == 0 {
		return false
	}
	w := before[len(before)-1].lower
	if comparatives.Contains(w) && len(before) > 1 {
		w = before[len(before)-2].lower
	}
	return strings.HasSuffix(w, "'s") || strings.HasSuffix(w, "’s") ||
		cardinal.InClass(w, "article") || cardinal.InClass(w, "determiner")
}

// comparatives sit between a determiner and its number: "the same three rules".
var comparatives = set.Of[string]("same", "other")

// word is a blank-separated word in front of the number, on its own line.
type word struct {
	lower string
	start int
}

var wordRun = regexp.MustCompile(`\S+`)

// wordsBefore returns the words of the line in front of at, back to the last
// clause break. A word that ends a clause belongs to the clause before.
func wordsBefore(content string, at int) []word {
	lineStart := strings.LastIndexByte(content[:at], '\n') + 1
	line := content[lineStart:at]
	var out []word
	for _, loc := range wordRun.FindAllStringIndex(line, -1) {
		text := line[loc[0]:loc[1]]
		if strings.ContainsAny(text[len(text)-1:], ",;:.!?()[]\"") {
			out = out[:0]
			continue
		}
		out = append(out, word{lower: strings.ToLower(text), start: lineStart + loc[0]})
	}
	return out
}

var listMarker = regexp.MustCompile(`^\s*(?:[-*+>]|\d+[.)])?\s*$`)

// opensSentence reports whether at starts a sentence, so a digit there hands
// its capital to what replaces it.
func opensSentence(content string, at int) bool {
	lineStart := strings.LastIndexByte(content[:at], '\n') + 1
	prefix := content[lineStart:at]
	if trimmed := strings.TrimSpace(prefix); trimmed != "" {
		return strings.ContainsAny(trimmed[len(trimmed)-1:], ".!?")
	}
	if !listMarker.MatchString(prefix) {
		return false
	}
	if lineStart == 0 {
		return true
	}
	prevStart := strings.LastIndexByte(content[:lineStart-1], '\n') + 1
	prev := strings.TrimSpace(content[prevStart : lineStart-1])
	return prev == "" || strings.ContainsAny(prev[len(prev)-1:], ".!?") || strings.TrimSpace(prefix) != ""
}

var numberWords = map[string]int{
	"two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8,
	"nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13,
	"fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18,
	"nineteen": 19, "twenty": 20, "thirty": 30, "forty": 40, "fifty": 50,
	"sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90, "dozen": 12,
}

// value reads a cardinal as an integer. A number too large to read is large.
func value(number string) int {
	if n, ok := numberWords[strings.ToLower(number)]; ok {
		return n
	}
	n, err := strconv.Atoi(strings.ReplaceAll(number, ",", ""))
	if err != nil {
		return 1 << 30
	}
	return n
}

func lastField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}
