// Package laziness finds a turn ending because the model would rather not do
// the work.
//
// It folds together what used to be judged apart. A closing message reports a
// defect the session found and left in place. Or it asks permission to carry on
// instead of carrying on. Both end the turn with the work undone, and the reader
// is left holding it.
//
// The consumer is a Stop hook. It answers a finding with the single word the
// user would have typed back. There is nothing in that answer to argue with.
package laziness

import (
	"regexp"
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
	// Sentence quotes what was actually written.
	Sentence string `json:"sentence"`
	// Line is where the sentence starts, counting from the top.
	Line int `json:"line"`
}

// tell is a recognisable shape. name is what a report prints.
type tell struct {
	name string
	re   *regexp.Regexp
}

// tells is the table. It is data: extending this package is adding a row.
//
// Each row is a phrase a punt is really written in, kept narrow enough that
// ordinary reporting does not trip it. A bare diagnosis of a pre-existing
// failure is legitimate, so that word alone is absent here. What is caught is
// the diagnosis offered as the reason for stopping.
var tells = []tell{
	{"disowning a defect you found", regexp.MustCompile(`(?i)\b(?:not|neither)\s+(?:mine|ours|my own)\s+to\s+fix\b`)},
	{"disowning a defect you found", regexp.MustCompile(`(?i)\bnot\s+(?:my|our)\s+(?:problem|job|responsibility)\b`)},

	{"handing a repair back to the reader", regexp.MustCompile(`(?i)\bworth\s+your\s+attention\b`)},
	{"handing a repair back to the reader", regexp.MustCompile(`(?i)\bflagging\s+(?:it|this|these|them|that)?\s*(?:for|to)\s+you\b`)},
	{"handing a repair back to the reader", regexp.MustCompile(`(?i)\bsomeone\s+should\s+(?:fix|look|clean|deal)\b`)},
	{"handing a repair back to the reader", regexp.MustCompile(`(?i)\byou\s+(?:may|might|could)\s+want\s+to\s+(?:fix|look|check|clean)\b`)},

	{"leaving a defect in place", regexp.MustCompile(`(?i)\bleft\s+(?:it|them|that|these|those)\s+(?:as[- ]is|alone|unfixed|broken|red)\b`)},
	{"leaving a defect in place", regexp.MustCompile(`(?i)\bleaving\s+(?:it|them|that|these|those)\s+(?:as[- ]is|alone|unfixed|broken|red)\b`)},
	{"leaving a defect in place", regexp.MustCompile(`(?i)\bI\s+(?:did\s+not|didn['’]t)\s+fix\b`)},

	{"excusing yourself from a repair", regexp.MustCompile(`(?i)\bunasked\b`)},
	{"excusing yourself from a repair", regexp.MustCompile(`(?i)\bout\s+of\s+scope\b`)},
	{"excusing yourself from a repair", regexp.MustCompile(`(?i)\boutside\s+(?:the|my|our)\s+scope\b`)},
	{"excusing yourself from a repair", regexp.MustCompile(`(?i)\bnot\s+caused\s+by\s+(?:my|this|the)\s+change\b`)},
	{"excusing yourself from a repair", regexp.MustCompile(`(?i)\bpre-?existing\b[^.!?\n]{0,80}\b(?:so|and)\s+(?:I|we)\s+(?:did\s+not|didn['’]t|left|have\s+not|haven['’]t)\b`)},

	// Attribution in place of repair, narrow on purpose.
	{"announcing an attribution hunt instead of a repair", regexp.MustCompile(`(?i)\b(?:two|three|first)\s+things?\s+to\s+establish\b`)},
	{"announcing an attribution hunt instead of a repair", regexp.MustCompile(`(?i)\bwhether\s+(?:master|main|the\s+base\s+branch)\s+is\s+(?:also\s+)?(?:red|failing|broken)\b`)},
	{"announcing an attribution hunt instead of a repair", regexp.MustCompile(`(?i)\b(?:establish|determine|work\s+out|figure\s+out)\s+(?:whether|if|who)\s+(?:I|we|this\s+PR|my\s+change)\b`)},
	{"offering authorship in place of a fix", regexp.MustCompile(`(?i)\bnot\s+(?:this\s+PR|my\s+change|mine)['’]?s?\s+(?:to\s+)?(?:fault|failure|problem)\b`)},
	{"offering authorship in place of a fix", regexp.MustCompile(`(?i)\b(?:fails|red|broken|failing)\s+on\s+(?:master|main|the\s+base\s+branch)\s+too\b[^.!?\n]{0,60}\b(?:so|therefore)\b`)},
	{"offering authorship in place of a fix", regexp.MustCompile(`(?i)\bI\s+(?:did\s+not|didn['’]t)\s+(?:break|cause)\s+(?:it|this|that)\b`)},

	// The turn ends on a question whose answer was already yes.
	{"asking permission in place of acting", regexp.MustCompile(`(?i)\bwant\s+me\s+to\b[^.!?\n]*\?`)},
	{"asking permission in place of acting", regexp.MustCompile(`(?i)\bwould\s+you\s+like\s+me\s+to\b[^.!?\n]*\?`)},
	{"asking permission in place of acting", regexp.MustCompile(`(?i)\bshall\s+I\b[^.!?\n]*\?`)},
	{"asking permission in place of acting", regexp.MustCompile(`(?i)\bshould\s+I\b[^.!?\n]*\?`)},
	{"asking permission in place of acting", regexp.MustCompile(`(?i)\bdo\s+you\s+want\s+me\s+to\b[^.!?\n]*\?`)},
	{"asking permission in place of acting", regexp.MustCompile(`(?i)\blet\s+me\s+know\s+if\s+you\s*(?:'|’)?d?\s*(?:would\s+)?like\b`)},
	{"asking permission in place of acting", regexp.MustCompile(`(?i)\bI\s+can\s+\S[^.!?\n]{0,60}\s+if\s+you\s*(?:'|’)?d?\s*(?:would\s+)?like\b`)},
	{"asking permission in place of acting", regexp.MustCompile(`(?i)\bsay\s+the\s+word\b`)},
}

// pardons mean the obligation was met. A sentence carrying a tell AND saying
// the repair happened is not a punt.
var pardons = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:fixed|repaired|patched|corrected)\s+(?:it|them|that|those|these|anyway|here|now|in)\b`),
	regexp.MustCompile(`(?i)\balready\s+(?:fixed|repaired|pushed|landed|corrected)\b`),
	regexp.MustCompile(`(?i)\b(?:pushed|landed|committed)\s+(?:the\s+)?fix\b`),
	regexp.MustCompile(`(?i)\bfix(?:ed)?\s+(?:it\s+)?in\s+(?:this|the\s+same)\s+(?:turn|commit|change|PR|pull\s+request)\b`),
}

// Check reports every punt the message carries.
//
// A sentence is reported for a single tell, whichever found it. A message
// repeating itself must not read as a worse offence than it is.
func Check(message string) []Hit {
	text, kept := assertedText(message)
	var hits []Hit
	seen := map[string]bool{}
	for _, t := range tells {
		at := t.re.FindStringIndex(text)
		if at == nil {
			continue
		}
		start, end := sentenceBounds(text, at[0])
		sentence := strings.Join(strings.Fields(text[start:end]), " ")
		if sentence == "" || seen[sentence] || isPardoned(sentence) {
			continue
		}
		seen[sentence] = true
		hits = append(hits, Hit{ID: ID, Tell: t.name, Sentence: sentence, Line: lineOf(kept, start)})
	}
	return hits
}

func isPardoned(sentence string) bool {
	for _, re := range pardons {
		if re.MatchString(sentence) {
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

// lineOf answers which source line a byte offset in the asserted text came from.
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
