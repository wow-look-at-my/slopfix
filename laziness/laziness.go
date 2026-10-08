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
	"regexp"
	"sort"
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

// tell is a recognisable shape. name is what a report prints.
type tell struct {
	name string
	re   *regexp.Regexp
}

// tells is the table. It is data: extending this package is adding a row. Each
// row is a phrase a punt is written in, narrow enough that ordinary reporting
// does not trip it.
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

// repairs runs parallel to tells: one row answers each row of the table. It is
// data too, so a tell added without a repair is a length mismatch the package's
// own test catches.
var repairs = []repair{
	{words: "mine to fix"},
	{words: "mine to fix"},

	{words: "mine to fix"},
	{words: "fixing it"},
	{words: "I will fix"},
	{words: "I will fix"},

	{words: "fixed it"},
	{words: "fixing it"},
	{words: "I will fix"},

	{words: "now"},
	{words: "mine to fix"},
	{words: "mine to fix"},
	{words: "mine to fix"},
	{words: "mine to fix; I will"},

	{words: "one fix to make"},
	{words: "what else is failing"},
	{words: "I will own that I"},
	{words: "mine to fix"},
	{words: "is mine to fix and"},
	{words: "I will fix it"},

	{question: true},
	{question: true},
	{question: true},
	{question: true},
	{question: true},
	{question: true},
	{question: true},
	{question: true},
}

// repair is the wording one row of tells puts in place of its match. A
// question row states the action the question named instead.
type repair struct {
	// words replaces the matched span of a phrase tell.
	words string
	// question marks a permission tell, whose repair names its action.
	question bool
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
// A sentence is reported for a single tell, whichever found it.
func Check(message string) []Hit {
	text, kept := assertedText(message)
	var hits []Hit
	seen := set.New[string]()
	for _, t := range tells {
		at := t.re.FindStringIndex(text)
		if at == nil {
			continue
		}
		start, end := sentenceBounds(text, at[0])
		sentence := strings.Join(strings.Fields(text[start:end]), " ")
		if sentence == "" || seen.Contains(sentence) || isPardoned(sentence) {
			continue
		}
		seen.Add(sentence)
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

// Repair rewrites each sentence that carries a tell, so the repaired message
// owns the work instead of handing it back. Every byte outside a matched span
// keeps its place, and a pardoned sentence is left as written.
func Repair(message string) string {
	if message == "" {
		return message
	}
	out := message
	for range len(tells) + 1 {
		text, _ := assertedText(out)
		edits := repairSpans(text)
		if len(edits) == 0 {
			break
		}
		next := out
		for i := len(edits) - 1; i >= 0; i-- {
			e := edits[i]
			next = next[:e.start] + e.words + next[e.end:]
		}
		if next == out {
			break
		}
		out = next
	}
	return out
}

// edit is one matched span, with the wording that replaces it.
type edit struct {
	start int
	end   int
	words string
}

// repairSpans collects every tell match that is not pardoned, in table order,
// and drops an occurrence nested inside another so no span is rewritten twice.
func repairSpans(text string) []edit {
	var edits []edit
	for n, t := range tells {
		if n >= len(repairs) {
			break
		}
		for _, at := range t.re.FindAllStringIndex(text, -1) {
			start, end := sentenceBounds(text, at[0])
			sentence := strings.Join(strings.Fields(text[start:end]), " ")
			if sentence == "" || isPardoned(sentence) {
				continue
			}
			matched := text[at[0]:at[1]]
			words := repairs[n].words
			if repairs[n].question {
				words = questionWords(matched)
			} else {
				words = matchCase(matched, words)
			}
			if words == "" {
				continue
			}
			edits = append(edits, edit{start: at[0], end: at[1], words: words})
		}
	}
	return withoutOverlap(edits)
}

// withoutOverlap keeps an earlier span over one that overlaps it.
func withoutOverlap(edits []edit) []edit {
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start < edits[j].start
		}
		return edits[i].end > edits[j].end
	})
	var kept []edit
	for _, e := range edits {
		if len(kept) > 0 && e.start < kept[len(kept)-1].end {
			continue
		}
		kept = append(kept, e)
	}
	return kept
}

// matchCase keeps the source's initial capital, so a phrase that opened a
// sentence still opens one.
func matchCase(matched, words string) string {
	if matched == "" || words == "" {
		return words
	}
	if matched[0] >= 'A' && matched[0] <= 'Z' {
		return strings.ToUpper(words[:1]) + words[1:]
	}
	return words
}

// openers begin a permission question. The action follows the opener.
var openers = []string{
	"would you like me to ",
	"do you want me to ",
	"want me to ",
	"shall i ",
	"should i ",
	"i can ",
	"let me know if you'd like",
	"let me know if you would like",
	"let me know if you",
	"let me know",
	"say the word",
}

// invitations close a permission question without naming an action.
var invitations = []string{
	" if you'd like",
	" if you would like",
	" if you like",
}

// questionWords states the action a permission question handed back. It drops
// the opener, the invitation and the question mark, and makes no claim when the
// question named no action.
func questionWords(matched string) string {
	// Only a question mark is consumed.
	asked := strings.HasSuffix(strings.TrimSpace(matched), "?")
	action := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(matched), "?"))
	lower := strings.ToLower(action)
	for _, tail := range invitations {
		if strings.HasSuffix(lower, tail) {
			action = strings.TrimSpace(action[:len(action)-len(tail)])
			lower = strings.ToLower(action)
			break
		}
	}
	for _, head := range openers {
		if strings.HasPrefix(lower, head) {
			action = strings.TrimSpace(action[len(head):])
			break
		}
	}
	action = strings.Join(strings.Fields(action), " ")
	if action == "" {
		action = "do it"
	}
	statement := "I will " + action
	if asked {
		statement += "."
	}
	return statement
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
