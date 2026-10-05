package counts

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/syntax"
)

// Constants answers the name of a constant whose value is the digits n, or "".
type Constants func(n string) string

var leadingDigits = regexp.MustCompile(`^\d+`)

// determiners stand before a label, and go when the words that replace it bring their own.
var determiners = set.Of("the", "a", "an", "this", "that")

// LabelAt answers the edit for digits at byte start that name an item, as in
// "branch 3 sees", or set a point, as in "at 100 cols". It answers false for
// a number that counts. The item takes the words the text gives it elsewhere,
// such as "branch (3) the keyless fallback". A point takes the name of a
// constant with its value. With neither, a generic phrase takes the place.
func LabelAt(content string, start int, consts Constants) (edit.Edit, bool) {
	digits := leadingDigits.FindString(content[start:])
	if digits == "" {
		return edit.Edit{}, false
	}
	from := strings.LastIndexByte(content[:start], '\n') + 1
	to := len(content)
	if i := strings.IndexByte(content[start:], '\n'); i >= 0 {
		to = start + i
	}
	line := content[from:to]
	if cardinal.NamesAnItem(line, start-from) {
		return itemEdit(content, from, line, start-from, digits)
	}
	if prep, ok := cardinal.APoint(line, start-from); ok {
		return pointEdit(content, start, digits, prep, consts), true
	}
	return edit.Edit{}, false
}

// itemEdit writes the words for the item a label names. A plural noun after
// the label is what the label describes: "rule 6 inputs" becomes "the inputs
// of a later rule".
func itemEdit(content string, from int, line string, at int, digits string) (edit.Edit, bool) {
	words := syntax.Parse(line, nil).Words
	i := 0
	for i < len(words) && words[i].Start != at {
		i++
	}
	if i == 0 || i == len(words) {
		return edit.Edit{}, false
	}
	noun := strings.TrimPrefix(words[i-1].Text, "(")
	start := from + words[i-1].End - len(noun)
	end := from + words[i].End
	determined := i >= 2 && determiners.Contains(words[i-2].Lower())
	if determined {
		start = from + words[i-2].Start
	}
	name := resolveItem(content, noun, digits)
	if name == "" {
		name = genericItem(noun, value(digits))
	}
	text := name
	if i+1 < len(words) && (words[i+1].Tag == "NNS" || words[i+1].Tag == "NNPS") {
		end = from + words[i+1].End
		text = "the " + words[i+1].Text + " of " + name
	}
	if opensSentence(content, start) {
		text = upperFirst(text)
	}
	return edit.Edit{Start: start, End: end, Text: text, Cut: []string{content[start:end]}}, true
}

// genericItem names an item with no words of its own: "one id", "a later branch".
func genericItem(noun string, n int) string {
	if !isUpper(noun) {
		noun = strings.ToLower(noun)
	}
	if n <= 1 {
		return "one " + noun
	}
	return "a later " + noun
}

// resolveItem answers the words the text gives an item elsewhere, or "". It
// reads "branch (3) the keyless fallback" and "Keyless fallback (branch 3)".
func resolveItem(content, noun, digits string) string {
	n := regexp.QuoteMeta(noun)
	after := regexp.MustCompile(`(?i)\b` + n + `[ \t]*(?:\([ \t]*` + digits + `[ \t]*\)|` + digits + `[ \t]*[:—–])[ \t]*(the[ \t]+[^,;:.()\n—–]+)`)
	if m := after.FindStringSubmatch(content); m != nil {
		if name := nameOf(m[1]); name != "" {
			return name
		}
	}
	before := regexp.MustCompile(`(?m)^[ \t]*(?:#+[ \t]*)?(?:[-*+][ \t]+)?(?:\d+[.)][ \t]*)?([A-Z][\w'-]*(?:[ \t]+[a-z][\w'-]*){0,3})[ \t]*\([ \t]*(?i:` + n + `)[ \t]*\(?` + digits + `\)?[ \t]*\)`)
	if m := before.FindStringSubmatch(content); m != nil {
		return "the " + lowerFirst(m[1])
	}
	return ""
}

// nameOf trims a description to the name it opens with. A code span after a
// couple of words or more goes, because the words already name the item.
func nameOf(desc string) string {
	desc = strings.TrimSpace(desc)
	if tick := strings.IndexByte(desc, '`'); tick >= 0 {
		head := strings.Fields(desc[:tick])
		if len(head) >= 3 {
			desc = strings.Join(head, " ")
		} else if close := strings.IndexByte(desc[tick+1:], '`'); close >= 0 {
			desc = desc[:tick+1+close+1]
		} else {
			return ""
		}
	}
	if fields := strings.Fields(desc); len(fields) < 2 || len(fields) > 6 {
		return ""
	}
	return desc
}

// pointEdit writes the point a number sets. A constant with that value names
// it. Otherwise a plural noun after it takes "a set number of", and a bare
// point takes "a set limit", "a set value" or "a set amount".
func pointEdit(content string, start int, digits, prep string, consts Constants) edit.Edit {
	end := start + len(digits)
	if consts != nil {
		if name := consts(digits); name != "" {
			return edit.Edit{Start: start, End: end, Text: "`" + name + "`", Cut: []string{digits}}
		}
	}
	// A hedge goes with the number: "at exactly 850" is no set value.
	before := wordsBefore(content, start)
	for len(before) > 0 && approximations.Contains(before[len(before)-1].lower) {
		start = before[len(before)-1].start
		before = before[:len(before)-1]
	}
	rest := strings.Fields(content[end:])
	text := map[string]string{"to": "a set limit", "at": "a set value", "by": "a set amount"}[prep]
	if len(rest) > 0 && pluralNoun(content, end) {
		text = "a set number of"
	}
	return edit.Edit{Start: start, End: end, Text: text, Cut: []string{content[start:end]}}
}

// pluralNoun reports a plural noun as the word after byte end. The tagger
// reads a verb in "-s" there as a noun, as in "truncating to 15 cuts". So the
// word is a noun only where the sentence opens on the point, or has its verb
// before it.
func pluralNoun(content string, end int) bool {
	from := strings.LastIndexByte(content[:end], '\n') + 1
	to := len(content)
	if i := strings.IndexByte(content[end:], '\n'); i >= 0 {
		to = end + i
	}
	words := syntax.Parse(content[from:to], nil).Words
	opening, finite := 0, false
	for i, w := range words {
		if from+w.Start < end {
			switch {
			case w.Tag == ".":
				opening, finite = i+1, false
			case finiteVerbTags.Contains(w.Tag):
				finite = true
			}
			continue
		}
		if w.Tag != "NNS" && w.Tag != "NNPS" {
			return false
		}
		// The point is the number and the preposition before it.
		return finite || i-2 <= opening
	}
	return false
}

// finiteVerbTags mark a finite verb.
var finiteVerbTags = set.Of("VBZ", "VBD", "VBP", "MD")

func upperFirst(s string) string {
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+len(string(r)):]
	}
	return s
}

func lowerFirst(s string) string {
	if len(s) > 1 && unicode.IsUpper(rune(s[1])) {
		return s
	}
	for i, r := range s {
		return string(unicode.ToLower(r)) + s[i+len(string(r)):]
	}
	return s
}

func isUpper(s string) bool { return s == strings.ToUpper(s) && s != strings.ToLower(s) }
