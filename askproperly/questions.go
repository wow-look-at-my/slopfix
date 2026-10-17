// questions.go finds the shapes a closing message must not end on. One shape
// is a question put to the user in prose. The other is a deferral that hands
// the user a decision without AskUserQuestion.
//
// Both tables live in questions.xml. Extending either is adding an entry
// there, never touching the matcher.
package askproperly

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/precompiled"
)

// ID names this rule, on a report and on the command line alike.
const ID = "ask/prose-decision"

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

var deferralMatchers = compilePhrases(Rules.DeferralPhrases())

func compilePhrases(phrases []string) []phraseMatcher {
	out := make([]phraseMatcher, 0, len(phrases))
	for _, p := range phrases {
		out = append(out, phraseMatcher{
			text: p,
			re:   precompiled.MustCompile(`(?i)` + regexp.QuoteMeta(p)),
		})
	}
	return out
}

var cueSet = func() map[string]bool {
	m := make(map[string]bool, len(Rules.Cues))
	for _, w := range Rules.Cues {
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
