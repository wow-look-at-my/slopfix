// Package laziness finds a turn ending because the model would rather not do
// the work.
//
// It folds together what used to be judged apart. A closing message reports a
// defect the session found and left in place. Or it asks permission to carry
// on instead of carrying on. Both end the turn with the work undone, and the
// reader is left holding it.
//
// The consumer is a Stop hook. It answers a finding with the word the user
// would have typed back.
package laziness

import (
	"github.com/wow-look-at-my/go-containers/set"
	"strings"
)

// ID names this rule, on a report and on the command line alike.
const ID = "laziness/punt"

// Hit is a punt found in a message.
type Hit struct {
	// ID names the rule, the way a compiler names a warning.
	ID string `json:"id"`
	// Tell says in words which shape fired.
	Tell string `json:"tell"`
	// Sentence quotes what was written.
	Sentence string `json:"sentence"`
	// Line is where the sentence starts, counting from the top.
	Line int `json:"line"`
}

// Check reports every punt the message carries.
//
// A sentence is reported for a single tell, whichever found it.
func Check(message string) []Hit {
	text, kept := assertedText(message)
	var hits []Hit
	seen := set.New[string]()
	for _, t := range tells {
		at := t.Re.FindStringIndex(text)
		if at == nil {
			continue
		}
		start, end := sentenceBounds(text, at[0])
		sentence := strings.Join(strings.Fields(text[start:end]), " ")
		if sentence == "" || seen.Contains(sentence) || isPardoned(sentence) {
			continue
		}
		seen.Add(sentence)
		hits = append(hits, Hit{ID: ID, Tell: t.Shape, Sentence: sentence, Line: lineOf(kept, start)})
	}
	return hits
}

func isPardoned(sentence string) bool {
	for _, p := range pardons {
		if p.Re.MatchString(sentence) {
			return true
		}
	}
	return false
}

// sentenceBounds returns the sentence a match sits inside, so a report quotes
// what was written rather than a regexp.
func sentenceBounds(text string, index int) (start, end int) {
	for i := index - 1; i >= 0; i-- {
		if endsSentence(text, i) {
			start = i + 1
			break
		}
	}
	end = len(text)
	for i := index; i < len(text); i++ {
		if endsSentence(text, i) {
			end = i + 1
			break
		}
	}
	return start, end
}

// endsSentence reports a character that closes the sentence before it.
//
// A period flanked by alphanumerics sits INSIDE a token, as in CLAUDE.md.
func endsSentence(text string, at int) bool {
	switch text[at] {
	case '\n', '!', '?':
		return true
	case '.':
		return !(isAlnum(byteAt(text, at-1)) && isAlnum(byteAt(text, at+1)))
	}
	return false
}

func byteAt(text string, at int) byte {
	if at < 0 || at >= len(text) {
		return 0
	}
	return text[at]
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func lineOf(kept []int, at int) int {
	if at < len(kept) {
		return kept[at]
	}
	if len(kept) > 0 {
		return kept[len(kept)-1]
	}
	return 1
}

// assertedText blanks what the message quotes rather than asserts, keeping every
// byte offset so a report still names the right line.
//
// Fenced code, indented code and a blockquote are exempt. This policy can then
// be written down without tripping the rule. An inline backtick span is NOT
// exempt, matching the sibling guards: a punt in backticks is still the writer's
// own voice.
func assertedText(message string) (text string, lines []int) {
	var b strings.Builder
	fenced := false
	for n, line := range strings.Split(message, "\n") {
		trimmed := strings.TrimSpace(line)
		fence := strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
		if fence {
			fenced = !fenced
		}
		quoted := fence || fenced ||
			strings.HasPrefix(trimmed, ">") ||
			strings.HasPrefix(line, "    ") ||
			strings.HasPrefix(line, "\t")
		if quoted {
			line = strings.Repeat(" ", len(line))
		}
		if n > 0 {
			b.WriteByte('\n')
			lines = append(lines, n+1)
		}
		b.WriteString(line)
		for range len(line) {
			lines = append(lines, n+1)
		}
	}
	return b.String(), lines
}
