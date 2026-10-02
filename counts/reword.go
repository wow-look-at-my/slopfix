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
	case len(before) > 0 && floorsAbove.Contains(last(1)):
		drop(1)
		text = vague(n + 1)
	case determined(before):
		text = ""
	case unit:
		text = vague(n)
	case len(before) > 0 && (cardinal.InClass(last(1), "preposition") || followsANoun(content, hit.Start)):
		text = "multiple "
	}

	e := edit.Edit{Start: from, End: to, Text: text, Cut: []string{hit.Phrase}}
	first, _ := utf8.DecodeRuneInString(content[from:])
	capital := unicode.IsUpper(first) || (!unicode.IsLetter(first) && opensSentence(content, from))
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
	"less than", "not more than", "under", "below", "within")

// floors set a minimum the vague word meets.
var floors = set.Of[string]("at least", "no fewer than", "no less than", "not less than")

// floorsAbove exclude the number itself, so the vague word reads one higher.
var floorsAbove = set.Of[string]("more than", "over", "above")

// determined reports whether a determiner or a possessive governs the noun
// already, as in "the three rules" or "this repo's plugins". The number then
// goes alone.
func determined(before []word) bool {
	if len(before) == 0 {
		return false
	}
	w := before[len(before)-1].lower
	return strings.HasSuffix(w, "'s") || strings.HasSuffix(w, "’s") ||
		cardinal.InClass(w, "article") || cardinal.InClass(w, "determiner")
}

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
