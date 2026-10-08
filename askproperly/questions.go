// questions.go finds the shapes a closing message must not end on. One shape
// is a question put to the user in prose. The other is a deferral that hands
// the user a decision without AskUserQuestion.
//
// Both tables are data on purpose. Extending either is editing a slice,
// never touching the matcher.
package askproperly

import (
	"github.com/wow-look-at-my/go-containers/set"
	"regexp"
	"sort"
	"strings"
)

// ID names this rule, on a report and on the command line alike.
const ID = "ask/prose-decision"

// deferralPhrases is the phrase table for handing a decision back without a
// question mark. Each entry offloads a choice the model was asked to make. A
// message may state what it did and stop; it may not close by inviting the
// user to decide in prose.
var deferralPhrases = []string{
	"let me know",
	"your call",
	"up to you",
	"say the word",
	"just say",
	"tell me which",
	"tell me what you",
	"waiting on you",
	"waiting for you to",
	"if you want me to",
	"want me to",
	"would you like me to",
	"do you want me to",
	"shall i",
	"should i proceed",
	"i won't touch",
	"i will not touch",
	"and i'll pick",
	"and i will pick",
	"and i'll land",
	"or say",
}

// owningPhrase is the repair for a deferral phrase: the words that take the
// decision the phrase handed over. A phrase with no entry cannot be repaired,
// so every entry in deferralPhrases needs one.
//
// No replacement carries a deferral phrase of its own. "and i'll pick" cannot
// become "and I will pick", because "and i will pick" is itself in the table
// and the repaired text would be found again.
var owningPhrase = map[string]string{
	"let me know":          "I will do it",
	"your call":            "mine to decide",
	"up to you":            "mine to decide",
	"say the word":         "I will do it",
	"just say":             "I will do it",
	"tell me which":        "I will decide which",
	"tell me what you":     "I will decide what you",
	"waiting on you":       "mine to do",
	"waiting for you to":   "mine to do",
	"if you want me to":    "I will",
	"want me to":           "I will",
	"would you like me to": "I will",
	"do you want me to":    "I will",
	"shall i":              "I will",
	"should i proceed":     "I will proceed",
	"i won't touch":        "I will handle",
	"i will not touch":     "I will handle",
	"and i'll pick":        "and I will choose",
	"and i will pick":      "and I will choose",
	"and i'll land":        "and I will land",
	"or say":               "or I will say",
}

// cueWords are the interrogative cues that separate a real question from a
// question mark doing another job. A "?" alone is not enough: nullable types
// and query strings both carry the mark, in a message that asked nothing.
var cueWords = []string{
	"what", "which", "why", "how", "when", "where", "who", "whose",
	"should", "shall", "would", "could", "can", "will", "do", "does",
	"did", "is", "are", "was", "were", "am", "have", "has", "any",
	"want", "prefer", "ok", "okay", "right", "correct", "agree", "sound",
}

// Hit is a finding, with the line it sits on so the refusal can quote it.
type Hit struct {
	Kind string // "question" or "deferral"
	Text string
	Line string
}

type phraseMatcher struct {
	text string
	re   *regexp.Regexp
}

var deferralMatchers = compilePhrases(deferralPhrases)

// deferralRepairs orders the phrases longest first, so a phrase that contains
// a shorter one claims its bytes before the shorter phrase reaches them.
var deferralRepairs = func() []string {
	phrases := append([]string(nil), deferralPhrases...)
	sort.Slice(phrases, func(i, j int) bool {
		if len(phrases[i]) != len(phrases[j]) {
			return len(phrases[i]) > len(phrases[j])
		}
		return phrases[i] < phrases[j]
	})
	return phrases
}()

func compilePhrases(phrases []string) []phraseMatcher {
	out := make([]phraseMatcher, 0, len(phrases))
	for _, p := range phrases {
		out = append(out, phraseMatcher{
			text: p,
			re:   regexp.MustCompile(`(?i)` + regexp.QuoteMeta(p)),
		})
	}
	return out
}

var cueSet = func() map[string]bool {
	m := make(map[string]bool, len(cueWords))
	for _, w := range cueWords {
		m[w] = true
	}
	return m
}()

// FindQuestions returns every question and deferral the message asserts in its
// own voice. An empty result allows the stop.
func FindQuestions(message string) []Hit {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	asserted := assertedText(message)
	scan, offsets := stripLinks(asserted)

	var hits []Hit
	seen := set.New[string]()

	for i := 0; i < len(scan); i++ {
		if scan[i] != '?' {
			continue
		}
		sentence := sentenceEndingAt(scan, i)
		if !closesAQuestion(scan, i, sentence) {
			continue
		}
		line := lineAt(asserted, offsets[i])
		if seen.Contains("q:" + line) {
			continue
		}
		seen.Add("q:" + line)
		hits = append(hits, Hit{Kind: "question", Text: strings.TrimSpace(sentence), Line: line})
	}

	for _, m := range deferralMatchers {
		loc := m.re.FindStringIndex(scan)
		if loc == nil {
			continue
		}
		line := lineAt(asserted, offsets[loc[0]])
		if seen.Contains("d:" + m.text) {
			continue
		}
		seen.Add("d:" + m.text)
		hits = append(hits, Hit{Kind: "deferral", Text: m.text, Line: line})
	}
	return hits
}

// sentenceEndingAt returns the sentence that the "?" at index i closes,
// bounded by the sentence terminator or line break.
func sentenceEndingAt(s string, i int) string {
	start := 0
	for j := i - 1; j >= 0; j-- {
		switch s[j] {
		case '.', '!', '?', '\n':
			start = j + 1
			j = -1
		}
	}
	sentence := s[start:i]
	if len(sentence) > 400 {
		sentence = sentence[len(sentence)-400:]
	}
	return sentence
}

// closesAQuestion decides whether the "?" at index i ends a question rather
// than spelling a nullable type.
//
// The line ending and the opening word decide it: a type annotation always has
// the thing it annotates after it.
func closesAQuestion(s string, i int, sentence string) bool {
	j := i + 1
	for j < len(s) && isTrailingCloser(s[j]) {
		j++
	}
	if j >= len(s) || s[j] == '\n' {
		return true
	}
	if !isASCIISpace(s[j]) {
		return false
	}
	return opensWithCue(sentence)
}

// isTrailingCloser reports a character that may sit between a question mark
// and the end of its line without changing that it ended the line.
func isTrailingCloser(c byte) bool {
	switch c {
	case '`', '"', '\'', ')', ']', '*', '_':
		return true
	}
	return false
}

// opensWithCue reports whether an interrogative cue sits in the opening words
// of a sentence. The window is deliberately narrow: "The field is ..." reaches
// "is" past it, and that sentence is a statement.
func opensWithCue(sentence string) bool {
	fields := strings.Fields(strings.ToLower(sentence))
	for n, f := range fields {
		if n >= 2 {
			return false
		}
		if cueSet[strings.Trim(f, "\"'`*_,;:()[]{}<>-")] {
			return true
		}
	}
	return false
}

// stripLinks blanks markdown link destinations and bare URLs, returning the
// scannable text plus, for every byte kept, its offset in the input. A
// compare URL's `expand` query is not a question, and neither is a query string
// in a bare link.
func stripLinks(text string) (string, []int) {
	var b strings.Builder
	offsets := make([]int, 0, len(text))
	for i := 0; i < len(text); {
		if text[i] == ']' && i+1 < len(text) && text[i+1] == '(' {
			if end := strings.IndexByte(text[i+1:], ')'); end >= 0 {
				b.WriteString("](")
				offsets = append(offsets, i, i+1)
				i += end + 1
				continue
			}
		}
		if strings.HasPrefix(text[i:], "http://") || strings.HasPrefix(text[i:], "https://") {
			j := i
			for j < len(text) && !isASCIISpace(text[j]) && text[j] != ')' && text[j] != '>' {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(text[i])
		offsets = append(offsets, i)
		i++
	}
	return b.String(), offsets
}

func isASCIISpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}

// assertedText drops what a message QUOTES rather than states: fenced code,
// indented code, and blockquotes. A message documenting this policy needs
// somewhere to write a question out.
//
// Inline backticks are NOT exempt, matching both siblings.
func assertedText(text string) string {
	var out []string
	fence := ""
	for _, line := range strings.Split(text, "\n") {
		if marker := fenceMarker(line); marker != "" {
			if fence == "" {
				fence = marker
			} else if marker == fence {
				fence = ""
			}
			out = append(out, "")
			continue
		}
		if fence != "" {
			out = append(out, "")
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, ">") {
			out = append(out, "")
			continue
		}
		if strings.HasPrefix(line, "    ") && trimmed != "" {
			out = append(out, "")
			continue
		}
		out = append(out, blankQuoted(line))
	}
	return strings.Join(out, "\n")
}

// blankQuoted blanks the inside of a double-quoted span, so a phrase a
// message MENTIONS is not read as a phrase it USES. The span keeps its
// length, so later offsets still point where they did.
func blankQuoted(line string) string {
	out := []byte(line)
	open := -1
	for i := 0; i < len(out); i++ {
		if out[i] != '"' {
			continue
		}
		if open < 0 {
			open = i
			continue
		}
		for j := open + 1; j < i; j++ {
			out[j] = ' '
		}
		open = -1
	}
	return string(out)
}

// fenceMarker returns "`" or "~" when a line opens or closes a code fence.
func fenceMarker(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	for _, m := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, m) {
			return m[:1]
		}
	}
	return ""
}

// lineAt returns the line containing byte offset i, collapsed and bounded so a
// refusal quotes a readable fragment rather than a paragraph.
func lineAt(text string, i int) string {
	if i < 0 || i > len(text) {
		return ""
	}
	start := strings.LastIndexByte(text[:i], '\n') + 1
	end := strings.IndexByte(text[i:], '\n')
	if end < 0 {
		end = len(text)
	} else {
		end += i
	}
	line := strings.Join(strings.Fields(text[start:end]), " ")
	if len(line) > 160 {
		line = line[:157] + "..."
	}
	return line
}

// Repair rewrites each question and each deferral into a statement that
// makes the decision, so the repaired message asks the reader for nothing.
func Repair(message string) string {
	if strings.TrimSpace(message) == "" {
		return message
	}
	out := message
	for pass := 0; pass < repairPasses; pass++ {
		next := repairOnce(out)
		if next == out {
			break
		}
		out = next
	}
	return out
}

// repairPasses bounds the loop that reruns a repair until the message stops changing.
const repairPasses = 4

// replacement is one span of the message and the text that takes its place.
type replacement struct {
	start int
	end   int
	text  string
}

// repairOnce makes one pass over the message. A question claims its whole
// sentence before any deferral does, so a deferral phrase inside a question is
// answered by the question's own statement.
func repairOnce(message string) string {
	mask := maskText(message)
	scan, offsets := stripLinks(mask)
	claimed := make([]bool, len(message))
	var reps []replacement

	for i := 0; i < len(scan); i++ {
		if scan[i] != '?' {
			continue
		}
		if !closesAQuestion(scan, i, sentenceEndingAt(scan, i)) {
			continue
		}
		start := sentenceStartAt(scan, i)
		start = questionStart(scan, start)
		mstart := offsets[start]
		mend := offsets[i] + 1
		if overlaps(claimed, mstart, mend) {
			continue
		}
		claim(claimed, mstart, mend)
		reps = append(reps, replacement{mstart, mend, questionStatement(message[mstart:mend])})
	}

	for _, phrase := range deferralRepairs {
		own := owningPhrase[phrase]
		if own == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(phrase))
		for _, loc := range re.FindAllStringIndex(scan, -1) {
			mstart := offsets[loc[0]]
			mend := offsets[loc[1]-1] + 1
			if overlaps(claimed, mstart, mend) {
				continue
			}
			claim(claimed, mstart, mend)
			reps = append(reps, replacement{mstart, mend, own})
		}
	}

	if len(reps) == 0 {
		return message
	}
	sort.Slice(reps, func(i, j int) bool { return reps[i].start < reps[j].start })

	var b strings.Builder
	last := 0
	for _, r := range reps {
		if r.start < last {
			continue
		}
		b.WriteString(message[last:r.start])
		b.WriteString(r.text)
		last = r.end
	}
	b.WriteString(message[last:])
	return b.String()
}

// sentenceStartAt returns the first byte of the sentence the "?" at index i
// closes, bounded by the sentence terminator or line break.
func sentenceStartAt(s string, i int) int {
	for j := i - 1; j >= 0; j-- {
		switch s[j] {
		case '.', '!', '?', '\n':
			return j + 1
		}
	}
	return 0
}

// questionStart reaches back across a line wrap. A wrapped question is one
// sentence, and the detector cuts it at the newline. The statement would
// otherwise be built from the second line alone.
func questionStart(s string, start int) int {
	for start > 0 && s[start-1] == '\n' {
		prev := sentenceStartAt(s, start-1)
		text := strings.TrimRight(s[prev:start-1], " \t")
		if text == "" || endsASentence(text[len(text)-1]) {
			break
		}
		start = prev
	}
	return start
}

// endsASentence reports a character that closes a sentence.
func endsASentence(c byte) bool {
	switch c {
	case '.', '!', '?':
		return true
	}
	return false
}

// questionStatement turns one question, with its "?", into the statement of the
// decision it named. The leading whitespace of the span stays where it was.
func questionStatement(span string) string {
	lead := span[:len(span)-len(strings.TrimLeft(span, " \t"))]
	body := strings.TrimSpace(strings.TrimSuffix(span, "?"))
	return lead + applyDeferrals(questionToStatement(body))
}

// applyDeferrals replaces every deferral phrase in generated text, so a
// statement built from a question cannot carry a phrase back in.
func applyDeferrals(text string) string {
	for _, phrase := range deferralRepairs {
		own := owningPhrase[phrase]
		if own == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(phrase))
		text = re.ReplaceAllLiteralString(text, own)
	}
	return text
}

// questionToStatement rewrites the words of a question as the decision they
// asked for. A question that names an action becomes "I will <action>"; a
// question that names a choice becomes a statement of the choice.
func questionToStatement(body string) string {
	words := strings.Fields(body)
	if len(words) == 0 {
		return "I will do it."
	}
	first := wordKey(words[0])
	if isAuxiliary(first) && len(words) >= 2 {
		subject := wordKey(words[1])
		switch {
		case subject == "i":
			if first == "am" {
				return beStatement("I", first, words[2:])
			}
			return actionStatement(words[2:])
		case subject == "you":
			if isBeVerb(first) {
				return beStatement("You", first, words[2:])
			}
			return youStatement(words[2:])
		case pronounSubject[subject]:
			return beStatement(capitalize(words[1]), first, words[2:])
		case first == "do" || first == "does" || first == "did":
			return capitalize(joinWords(words[1:])) + "."
		}
	}

	i := 0
	for i < len(words) && isCue(wordKey(words[i])) {
		i++
	}
	rest := words[i:]
	if len(rest) == 0 {
		return "I will do it."
	}
	if i == 0 {
		return capitalize(joinWords(rest)) + "."
	}
	switch subject := wordKey(rest[0]); {
	case subject == "i":
		return actionStatement(rest[1:])
	case subject == "you":
		return youStatement(rest[1:])
	case subject == "me" && len(rest) >= 2 && wordKey(rest[1]) == "to":
		return actionStatement(rest[2:])
	case determiner[subject]:
		return capitalize(joinWords(rest)) + "."
	}
	return "The " + joinWords(rest) + "."
}

// actionStatement writes the action a question named as the decision taken.
func actionStatement(rest []string) string {
	if len(rest) == 0 {
		return "I will do it."
	}
	return "I will " + joinWords(rest) + "."
}

// youStatement writes the decision a question aimed at the reader.
func youStatement(rest []string) string {
	if len(rest) == 0 {
		return "I will do it."
	}
	if wordKey(rest[0]) == "want" {
		if len(rest) >= 3 && wordKey(rest[1]) == "me" && wordKey(rest[2]) == "to" {
			return actionStatement(rest[3:])
		}
		if len(rest) >= 2 && wordKey(rest[1]) == "to" {
			return actionStatement(rest[2:])
		}
	}
	return "You " + joinWords(rest) + "."
}

// beStatement writes a subject and a be-verb as a statement, inverting the
// question back into subject-verb order.
func beStatement(subject, verb string, rest []string) string {
	if len(rest) == 0 {
		return subject + " " + verb + "."
	}
	return subject + " " + verb + " " + joinWords(rest) + "."
}

// isAuxiliary reports an auxiliary that a question may open with.
func isAuxiliary(w string) bool {
	return auxiliarySet[w]
}

var auxiliarySet = map[string]bool{
	"is": true, "are": true, "was": true, "were": true, "am": true,
	"do": true, "does": true, "did": true, "can": true, "will": true,
	"would": true, "could": true, "should": true, "shall": true,
	"have": true, "has": true,
}

// isBeVerb reports an auxiliary that carries no action of its own, so a
// question that opens with it keeps the verb in the statement.
func isBeVerb(w string) bool {
	switch w {
	case "is", "are", "was", "were", "am":
		return true
	}
	return false
}

// pronounSubject is a subject that a be-question can invert back into place.
var pronounSubject = map[string]bool{
	"it": true, "that": true, "this": true, "these": true, "those": true,
	"there": true, "he": true, "she": true, "they": true, "we": true,
}

// determiner is a word that already carries its own noun phrase, so a
// statement of a choice does not add another.
var determiner = map[string]bool{
	"the": true, "a": true, "an": true, "my": true, "your": true,
	"our": true, "their": true, "its": true, "this": true, "that": true,
	"these": true, "those": true,
}

// isCue reports an interrogative cue, the same table the detector reads.
func isCue(w string) bool { return cueSet[w] }

// wordKey folds a word for table lookup: lowercase, without surrounding
// punctuation.
func wordKey(w string) string {
	return strings.ToLower(strings.Trim(w, "\"'`*_,;:()[]{}<>-?."))
}

// joinWords writes words back as a phrase.
func joinWords(words []string) string { return strings.Join(words, " ") }

// capitalize uppercases the first letter of a phrase.
func capitalize(s string) string {
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] >= 'a' && s[i] <= 'z':
			return s[:i] + string(s[i]-'a'+'A') + s[i+1:]
		case s[i] >= 'A' && s[i] <= 'Z':
			return s
		}
	}
	return s
}

// overlaps reports whether any byte of the span is already claimed.
func overlaps(claimed []bool, start, end int) bool {
	for i := start; i < end && i < len(claimed); i++ {
		if claimed[i] {
			return true
		}
	}
	return false
}

// claim marks every byte of the span as replaced.
func claim(claimed []bool, start, end int) {
	for i := start; i < end && i < len(claimed); i++ {
		claimed[i] = true
	}
}

// maskText is assertedText with the exempt bytes blanked in place rather than
// dropped. Every byte of the result sits where it did in the message, so an
// offset into the mask is an offset into the message.
func maskText(text string) string {
	out := []byte(text)
	fence := ""
	lineStart := 0
	for i := 0; i <= len(text); i++ {
		if i < len(text) && text[i] != '\n' {
			continue
		}
		line := text[lineStart:i]
		if !blankMaskedLine(out, line, lineStart, &fence) {
			blankQuotedRange(out, lineStart, i)
		}
		lineStart = i + 1
	}
	return string(out)
}

// blankMaskedLine blanks a line the detector exempts, and reports whether it
// did. The fence state carries across lines.
func blankMaskedLine(out []byte, line string, start int, fence *string) bool {
	if marker := fenceMarker(line); marker != "" {
		if *fence == "" {
			*fence = marker
		} else if marker == *fence {
			*fence = ""
		}
		blankRange(out, start, start+len(line))
		return true
	}
	if *fence != "" {
		blankRange(out, start, start+len(line))
		return true
	}
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, ">") || (strings.HasPrefix(line, "    ") && trimmed != "") {
		blankRange(out, start, start+len(line))
		return true
	}
	return false
}

// blankRange writes spaces over a span, keeping every offset where it was.
func blankRange(out []byte, start, end int) {
	for i := start; i < end; i++ {
		out[i] = ' '
	}
}

// blankQuotedRange blanks the inside of a double-quoted span in place, so a
// phrase a message MENTIONS is not read as a phrase it USES.
func blankQuotedRange(out []byte, start, end int) {
	open := -1
	for i := start; i < end; i++ {
		if out[i] != '"' {
			continue
		}
		if open < 0 {
			open = i
			continue
		}
		for j := open + 1; j < i; j++ {
			out[j] = ' '
		}
		open = -1
	}
}
