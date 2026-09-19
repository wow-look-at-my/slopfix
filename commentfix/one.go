// one.go decides whether the cardinal `one` is a determiner or a pronoun.
//
// The repair for the determiner is "a single", which carries its own article.
// Written into a slot an article already governs, it spells "the a single". So
// the swap does not hold in every context, and rules/README.md admits only a
// swap that does. The decision needs the words on each side, which the table
// cannot read, so it lives here beside the word-boundary guard.
package commentfix

import "strings"

// governors are the words that cannot stand before "a single". A determiner
// already fills the slot the article needs, and a comparative adjective reads
// the cardinal after it as a pronoun.
var governors = map[string]bool{
	"the": true, "a": true, "an": true, "this": true, "that": true,
	"these": true, "those": true, "each": true, "every": true, "any": true,
	"no": true, "some": true, "another": true, "other": true, "which": true,
	"either": true, "neither": true, "its": true, "their": true, "our": true,
	"my": true, "your": true, "his": true, "her": true,
}

// pronounFollowers are the words that cannot head a noun phrase. The cardinal
// before such a word stands for a noun rather than counting it.
var pronounFollowers = map[string]bool{
	"that": true, "which": true, "who": true, "whom": true, "whose": true,
	"when": true, "where": true, "while": true, "if": true, "unless": true,
	"since": true, "because": true, "though": true, "although": true,
	"and": true, "or": true, "but": true, "so": true, "then": true,
	"still": true, "already": true, "only": true, "never": true,
	"always": true, "also": true, "too": true, "here": true, "there": true,
	"now": true, "of": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "with": true, "from": true, "by": true, "into": true,
	"onto": true, "under": true, "over": true, "per": true, "as": true,
	"than": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "has": true, "have": true,
	"had": true, "does": true, "do": true, "did": true, "can": true,
	"could": true, "will": true, "would": true, "may": true, "might": true,
	"must": true, "should": true,
}

// sayOne rewrites the determiner `one` and leaves the pronoun alone. A slot it
// cannot read confidently keeps the cardinal, which the rule then reports. A
// warning costs a reader nothing. A wrong repair ships prose nobody reviews.
func sayOne(prose string) string {
	var b strings.Builder
	lower := strings.ToLower(prose)
	for i := 0; i < len(prose); {
		at := indexWord(lower, i, "one")
		if at < 0 {
			b.WriteString(prose[i:])
			break
		}
		end := at + len("one")
		b.WriteString(prose[i:at])
		if determines(lower, at, end) {
			b.WriteString("a single")
		} else {
			b.WriteString(prose[at:end])
		}
		i = end
	}
	return b.String()
}

// determines reports whether the cardinal at s[at:end] counts the noun after
// it. It answers false wherever the reading is not plain.
func determines(s string, at, end int) bool {
	next := wordAfter(s, end)
	if next == "" || pronounFollowers[next] {
		return false
	}
	// A gerund reads as a verb here more often than as a noun, and telling the
	// two apart needs a parser.
	if strings.HasSuffix(next, "ing") {
		return false
	}
	prev := wordBefore(s, at)
	if governors[prev] || comparative(prev) {
		return false
	}
	return true
}

// comparative reports an adjective a cardinal cannot follow as a determiner,
// such as "smaller" or "largest".
func comparative(word string) bool {
	if len(word) < 5 {
		return false
	}
	return strings.HasSuffix(word, "er") || strings.HasSuffix(word, "est")
}

// indexWord answers where target stands as a whole word at or after from.
func indexWord(s string, from int, target string) int {
	for i := from; ; {
		j := strings.Index(s[i:], target)
		if j < 0 {
			return -1
		}
		at := i + j
		if wordBoundary(s, at, at+len(target)) {
			return at
		}
		i = at + 1
	}
}

// wordAfter answers the lowercased word following s[:from], or "" when the
// text runs out or something other than a letter comes next.
func wordAfter(s string, from int) string {
	i := from
	for i < len(s) && isSpaceByte(s[i]) {
		i++
	}
	start := i
	for i < len(s) && isWordByte(s[i]) {
		i++
	}
	if i == start || i == from {
		return ""
	}
	return s[start:i]
}

// wordBefore answers the lowercased word preceding s[at:], or "" when the text
// runs out or something other than a letter comes before.
func wordBefore(s string, at int) string {
	i := at
	for i > 0 && isSpaceByte(s[i-1]) {
		i--
	}
	end := i
	for i > 0 && isWordByte(s[i-1]) {
		i--
	}
	if i == end || end == at {
		return ""
	}
	return s[i:end]
}
