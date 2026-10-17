package counts

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/cardinal"
	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/ste"
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
	switch {
	case name != "":
	case value(digits) == 0:
		// Index zero is the first item, and the possessive before it already determines it: "the pack's first chunk".
		name = "the first " + lowerNoun(noun)
		if i >= 2 && !determined && possessive(words[i-2]) {
			name = "first " + lowerNoun(noun)
		}
	default:
		name = genericItem(noun, value(digits))
	}
	text := name
	// The word after the label is a noun it describes only where the clause has its verb before the label: "It reads rule 6 inputs", not "branch 3 sees".
	if i+1 < len(words) && (words[i+1].Tag == "NNS" || words[i+1].Tag == "NNPS") && verbBefore(words, i-1) {
		end = from + words[i+1].End
		text = "the " + words[i+1].Text + " of " + name
	}
	if opensSentence(content, start) {
		text = upperFirst(text)
	}
	return edit.Edit{Start: start, End: end, Text: text, Cut: []string{content[start:end]}}, true
}

// verbBefore reports a finite verb in the clause ahead of word i.
func verbBefore(words []syntax.Word, i int) bool {
	for j := i - 1; j >= 0; j-- {
		switch t := words[j].Tag; {
		case t == "." || t == "," || t == ":" || t == "(":
			return false
		case finiteVerbTags.Contains(t):
			return true
		}
	}
	return false
}

// genericItem names an item with no words of its own: "one id", "a later branch".
func genericItem(noun string, n int) string {
	noun = lowerNoun(noun)
	if n <= 1 {
		return "one " + noun
	}
	return "a later " + noun
}

// lowerNoun writes a noun in lower case, unless it is written in capitals.
func lowerNoun(noun string) string {
	if isUpper(noun) {
		return noun
	}
	return strings.ToLower(noun)
}

// possessive reports a word that determines the noun after it: "its", "the pack's".
func possessive(w syntax.Word) bool {
	lower := w.Lower()
	return w.Tag == "PRP$" || w.Tag == "POS" || strings.HasSuffix(lower, "'s") || strings.HasSuffix(lower, "s'")
}

// resolveItem answers the words the text gives an item elsewhere, or "". It
// reads "branch (3) the keyless fallback" and "Keyless fallback (branch 3)".
func resolveItem(content, noun, digits string) string {
	if desc, ok := describedAfter(content, noun, digits); ok {
		if name := nameOf(desc); name != "" {
			return name
		}
	}
	if name, ok := namedBefore(content, noun, digits); ok {
		return "the " + lowerFirst(name)
	}
	return ""
}

// labelThen reads what follows a label's noun: "(3) the keyless fallback" or
// "3: the keyless fallback". The digits are captured, so the pattern is constant.
var labelThen = regexp.MustCompile(`(?i)^[ \t]*(?:\([ \t]*(\d+)[ \t]*\)|(\d+)[ \t]*[:—–])[ \t]*(the[ \t]+[^,;:.()\n—–]+)`)

// describedAfter answers the description after the leftmost "noun (digits)"
// or "noun digits:" that starts at a word boundary. The noun matches without
// regard to case.
func describedAfter(content, noun, digits string) (string, bool) {
	for at := 0; at <= len(content); at++ {
		if at > 0 && at < len(content) && !utf8.RuneStart(content[at]) {
			continue
		}
		if !wordBoundary(content, at) {
			continue
		}
		n, ok := foldPrefix(content[at:], noun)
		if !ok {
			continue
		}
		m := labelThen.FindStringSubmatch(content[at+n:])
		if m == nil || m[1]+m[2] != digits {
			continue
		}
		return m[3], true
	}
	return "", false
}

// nameThenParen reads a line that names an item before a parenthesis. A name
// and the marks before it hold no "(", so the parenthesis is the first on the
// line, and each line has at most one reading.
var nameThenParen = regexp.MustCompile(`(?m)^[ \t]*(?:#+[ \t]*)?(?:[-*+][ \t]+)?(?:\d+[.)][ \t]*)?([A-Z][\w'-]*(?:[ \t]+[a-z][\w'-]*){0,3})[ \t]*\(`)

// namedBefore answers the name of the earliest line that reads
// "Name (noun digits)". The noun matches without regard to case.
func namedBefore(content, noun, digits string) (string, bool) {
	for _, m := range nameThenParen.FindAllStringSubmatchIndex(content, -1) {
		if closesLabel(content[m[1]:], noun, digits) {
			return content[m[2]:m[3]], true
		}
	}
	return "", false
}

// closesLabel reports whether s opens with `[ \t]*noun[ \t]*\(?digits\)?[ \t]*\)`,
// the noun without regard to case.
func closesLabel(s, noun, digits string) bool {
	s = strings.TrimLeft(s, " \t")
	n, ok := foldPrefix(s, noun)
	if !ok {
		return false
	}
	s = strings.TrimLeft(s[n:], " \t")
	s = strings.TrimPrefix(s, "(")
	if !strings.HasPrefix(s, digits) {
		return false
	}
	s = s[len(digits):]
	closes := func(t string) bool { return strings.HasPrefix(strings.TrimLeft(t, " \t"), ")") }
	return strings.HasPrefix(s, ")") && closes(s[1:]) || closes(s)
}

// foldPrefix answers the byte length of the prefix of s that matches lit rune
// by rune under Unicode case folding, as (?i) matches it.
func foldPrefix(s, lit string) (int, bool) {
	n := 0
	for _, want := range lit {
		got, size := utf8.DecodeRuneInString(s[n:])
		if size == 0 || !strings.EqualFold(string(got), string(want)) {
			return 0, false
		}
		n += size
	}
	return n, true
}

// wordBoundary reports a \b at byte at: an ASCII word byte on exactly one side.
func wordBoundary(s string, at int) bool {
	before := at > 0 && isWordByte(s[at-1])
	after := at < len(s) && isWordByte(s[at])
	return before != after
}

func isWordByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
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
	if ste.NameLike(s) {
		return s
	}
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
