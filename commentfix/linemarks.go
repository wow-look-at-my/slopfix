// linemarks.go reads the marks around a comment line's prose: a rule drawn
// with dashes, and the sentence ends a cut lands on.
package commentfix

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/ste"
)

// cutSentences splits comment prose where a cut may land. A sentence opens on
// a capital or on an identifier in lower case, such as cubeCount_.
func cutSentences(prose string) []string {
	return ste.SentencesOpenedBy(prose, func(rest []rune) bool {
		return ste.OpensSentence(rest) || opensOnIdentifier(rest)
	})
}

// opensOnIdentifier reports words that open on a name from code in lower case.
// Such a name holds a capital, a digit or an underscore after its first letter.
func opensOnIdentifier(rest []rune) bool {
	if len(rest) == 0 || !unicode.IsLower(rest[0]) {
		return false
	}
	for _, r := range rest[1:] {
		if unicode.IsSpace(r) {
			return false
		}
		if unicode.IsUpper(r) || unicode.IsDigit(r) || r == '_' {
			return true
		}
	}
	return false
}

// ruleChars are the characters a comment draws a rule with, as in "// ---- the probe ----".
const ruleChars = "-=*~#"

// cutRules splits the rule that opens a line's prose and the rule that closes
// it off the prose. The lead keeps the blank after its rule. A line that holds
// only a rule keeps it as its prose, because no prose sits beside it.
func cutRules(body string) (lead, prose, rule string) {
	prose = body
	if n := ruleRun(prose); n > 0 && n < len(prose) && (prose[n] == ' ' || prose[n] == '\t') {
		rest := strings.TrimLeft(prose[n:], " \t")
		lead, prose = prose[:len(prose)-len(rest)], rest
	}
	if at := strings.LastIndexAny(prose, " \t"); at >= 0 && ruleRun(prose[at+1:]) == len(prose)-at-1 {
		prose, rule = strings.TrimRight(prose[:at], " \t"), prose[at+1:]
	}
	return lead, prose, rule
}

// ruleRun answers the length of the rule that opens s, and zero when s opens no rule.
func ruleRun(s string) int {
	if s == "" || !strings.ContainsRune(ruleChars, rune(s[0])) {
		return 0
	}
	n := len(s) - len(strings.TrimLeft(s, s[:1]))
	if n < 3 {
		return 0
	}
	return n
}

// closesBlock reports a trailer that holds a block closer, and not only a rule.
func closesBlock(trailer string) bool {
	return strings.HasSuffix(trailer, "*/")
}
