// shorten.go applies the table. It is the whole of what a repair does to
// words, and it needs no parse: a caller hands it prose and gets prose back.
package english

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/syntax"
)

// danglingSpace matches the space a deletion leaves before punctuation that CLOSES something. A period with a word
var danglingSpace = regexp.MustCompile(`\s+([.,])(\s|$)`)

// doubledStop matches the period a deleted sentence leaves beside the period before it. An ellipsis does not match.
var doubledStop = regexp.MustCompile(`([^.])\.\.(\s|$)`)

// Surface names where prose is being read, which decides the entries that
// apply to it.
const (
	Comment  = "comment"
	Document = "document"
	Message  = "message"
)

// Fix rewrites prose with every table entry that names the surface.
func Fix(s, surface string) string {
	out, _ := FixN(s, surface)
	return out
}

// FixN is Fix, and the number of rewrites it took. A caller hands that count
// back to whoever wrote the prose, so the repair is visible rather than silent.
func FixN(s, surface string) (string, int) {
	original := s
	n := 0
	// Rewrites go before drops: a phrase like "in order to" would otherwise lose
	// its middle to a <drop> and stop matching as a phrase at all.
	for _, r := range Rewrites() {
		if AppliesTo(r.Where, surface) {
			var took int
			s, took = replaceWordN(s, r.From, r.To)
			n += took
		}
	}
	for _, d := range Drops() {
		if AppliesTo(d.Where, surface) {
			var took int
			s, took = replaceWordN(s, d.Word, "")
			n += took
		}
	}
	for _, sh := range Shapes() {
		if AppliesTo(sh.Where, surface) {
			var took int
			s, took = sh.ApplyN(s)
			n += took
		}
	}
	// Patterns last: they carry a shape rather than a phrase, and a shape must
	// see the text a word swap has already settled.
	for _, p := range Patterns() {
		if AppliesTo(p.Where, surface) {
			var took int
			s, took = p.ApplyN(s)
			n += took
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	s = danglingSpace.ReplaceAllString(s, "${1}${2}")
	s = doubledStop.ReplaceAllString(s, "${1}.${2}")
	if n > 0 {
		s = dropFragments(strings.Join(strings.Fields(original), " "), s)
	}
	return capitalise(original, s), n
}

// emptiedStop matches the stop a deleted sentence leaves after the stop before it: "it. The".
var emptiedStop = regexp.MustCompile(`([.!?])\s+\.(\s|$)`)

// sentenceRun matches a sentence and the blank after it. A stop inside a code span does not end one.
var sentenceRun = regexp.MustCompile("(?:`[^`]*`|[^.!?`])+[.!?]+(?:\\s+|$)|(?:`[^`]*`|[^.!?`])+$")

// dropFragments removes each sentence a cut left with no verb, where the
// sentence it came from had one: "example-plugin was deleted on purpose."
// leaves "example-plugin.", which says nothing. A sentence the cut emptied goes
// too. It compares the sentences pair by pair, so it does nothing when the cut
// changed their count.
func dropFragments(before, after string) string {
	if !strings.HasPrefix(before, ".") {
		after = strings.TrimLeft(after, " .")
	}
	after = emptiedStop.ReplaceAllString(after, "${1}${2}")
	was := sentenceRun.FindAllString(before, -1)
	now := sentenceRun.FindAllString(after, -1)
	// A single sentence is the table's own case, and dropping it would leave nothing.
	if len(was) < 2 || len(was) != len(now) {
		return after
	}
	var b strings.Builder
	dropped := false
	for i, sentence := range now {
		if !hasWordIn(sentence) || hasFiniteVerb(was[i]) && !hasFiniteVerb(sentence) {
			dropped = true
			continue
		}
		b.WriteString(sentence)
	}
	if !dropped {
		return after
	}
	return strings.TrimSpace(b.String())
}

// hasWordIn reports text that holds a letter.
func hasWordIn(text string) bool {
	return strings.IndexFunc(text, func(r rune) bool { return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' }) >= 0
}

// hasFiniteVerb reports a sentence that holds a finite verb or a modal.
func hasFiniteVerb(sentence string) bool {
	for _, w := range syntax.Parse(sentence, nil).Words {
		switch w.Tag {
		case "VBZ", "VBP", "VBD", "MD":
			return true
		}
	}
	return false
}

// ReplaceWord swaps a whole word or phrase, case-insensitively, leaving a
// longer word that merely contains it alone.
func ReplaceWord(s, word, with string) string {
	out, _ := replaceWordN(s, word, with)
	return out
}

func replaceWordN(s, word, with string) (string, int) {
	lower := strings.ToLower(s)
	target := strings.ToLower(word)
	took := 0
	spans := codeSpan.FindAllStringIndex(s, -1)
	quotes := quotation.FindAllStringIndex(s, -1)
	var b strings.Builder
	for i := 0; i < len(s); {
		j := strings.Index(lower[i:], target)
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		at := i + j
		end := at + len(target)
		if !WordBoundary(s, at, end) || inProperName(s, at, end) || insideAny(spans, at, end) || insideAny(quotes, at, end) {
			b.WriteString(s[i : at+1])
			i = at + 1
			continue
		}
		b.WriteString(s[i:at])
		b.WriteString(with)
		took++
		i = end
	}
	return b.String(), took
}

// inProperName reports a capitalized match inside a sentence with a
// capitalized word after it. Such a match is part of a name, and a name loses
// meaning when one of its words goes.
func inProperName(s string, at, end int) bool {
	if !isUpper(s[at]) {
		return false
	}
	before := strings.TrimRight(s[:at], " \t")
	if before == "" || strings.ContainsAny(before[len(before)-1:], ".!?:\n") {
		return false
	}
	after := strings.TrimLeft(s[end:], " \t")
	return after != "" && isUpper(after[0])
}

func isUpper(b byte) bool { return b >= 'A' && b <= 'Z' }

// ContainsWord reports a whole-word occurrence, so `hack` never matches
// `hackney` and `for now` never matches `for nowhere`.
func ContainsWord(s, word string) bool {
	for i := 0; ; {
		j := strings.Index(s[i:], word)
		if j < 0 {
			return false
		}
		at := i + j
		if WordBoundary(s, at, at+len(word)) {
			return true
		}
		i = at + 1
	}
}

// WordBoundary reports whether s[at:end] stands as its own word.
func WordBoundary(s string, at, end int) bool {
	if at > 0 && isWordByte(s[at-1]) {
		return false
	}
	if end < len(s) && isWordByte(s[end]) {
		return false
	}
	return true
}

func isWordByte(b byte) bool {
	return b == '_' || b == '-' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// capitalise restores the opening capital a leading deletion can remove.
func capitalise(original, s string) string {
	if s == "" || sameFirstWord(original, s) {
		return s
	}
	if c := s[0]; c >= 'a' && c <= 'z' {
		return string(c-32) + s[1:]
	}
	return s
}

// sameFirstWord reports whether the repair left the opening word in place.
func sameFirstWord(original, s string) bool {
	before, after := strings.Fields(original), strings.Fields(s)
	if len(before) == 0 || len(after) == 0 {
		return false
	}
	return before[0] == after[0]
}
