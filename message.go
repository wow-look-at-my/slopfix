package slopfix

import (
	"strings"

	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/slopfix/askproperly"
	"github.com/wow-look-at-my/slopfix/blamelanguage"
	"github.com/wow-look-at-my/slopfix/laziness"
	"github.com/wow-look-at-my/slopfix/ste"
)

// MessageIDs names every rule that judges a closing message, which is never
// on disk.
func MessageIDs() set.Set[string] {
	return set.Of(laziness.ID, blamelanguage.ID, askproperly.ID)
}

// CheckMessage judges a closing message with every message rule runs keeps.
func CheckMessage(message string, runs func(string) bool) []ste.Finding {
	var out []ste.Finding
	if runs(laziness.ID) {
		for _, hit := range laziness.Check(message) {
			out = append(out, ste.Finding{Line: hit.Line, ID: hit.ID, Rule: hit.Tell, Detail: hit.Sentence,
				Fix: "Do the work the sentence hands back. `slopfix fix --message` cuts the sentence."})
		}
	}
	if runs(blamelanguage.ID) {
		for _, hit := range blamelanguage.Check(message) {
			detail := hit.Phrase
			if detail == "" {
				detail = hit.Sentence
			}
			out = append(out, ste.Finding{Line: hit.Line, ID: hit.ID, Rule: hit.Tell, Detail: detail,
				Fix: "Own the defect and say what you fixed. `slopfix fix --message` cuts the sentence."})
		}
	}
	if runs(askproperly.ID) {
		for _, hit := range askproperly.FindQuestions(message) {
			out = append(out, ste.Finding{Line: messageLine(message, hit.Line), ID: askproperly.ID,
				Rule: "a decision handed to the reader in prose", Detail: strings.TrimSpace(hit.Text),
				Fix: "Make the decision and say what you assumed. `slopfix fix --message` cuts the question."})
		}
	}
	return Registered(out)
}

// FixMessage cuts each sentence a message rule reports, until the message
// carries no finding. A cut asserts nothing, where a rewrite of a punt or a
// question would have to claim work or a decision nobody made.
func FixMessage(message string, runs func(string) bool) string {
	for range len(message) + 1 {
		findings := CheckMessage(message, runs)
		if len(findings) == 0 {
			return message
		}
		next := cutSentences(message, findings, runs)
		if next == message {
			next = cutLines(message, findings)
		}
		message = next
	}
	return message
}

// cutSentences removes each sentence on a reported line that a message rule
// reports when it stands alone. A line no rule reported is never touched, so
// a quoted or fenced sentence the rules exempt stays.
func cutSentences(message string, findings []ste.Finding, runs func(string) bool) string {
	reported := set.New[int]()
	for _, f := range findings {
		reported.Add(f.Line)
	}
	var out strings.Builder
	last := 0
	for _, span := range messageSentences(message) {
		start, end := span[0], span[1]
		if !reported.Contains(lineAtOffset(message, start)) && !reported.Contains(lineAtOffset(message, end-1)) {
			continue
		}
		// The line break that ends a sentence also ends its line, so it stays.
		cut := end
		if message[cut-1] == '\n' {
			cut--
		}
		if len(CheckMessage(message[start:cut], runs)) == 0 {
			continue
		}
		out.WriteString(message[last:start])
		last = cut
	}
	out.WriteString(message[last:])
	return out.String()
}

// cutLines removes each reported line.
func cutLines(message string, findings []ste.Finding) string {
	reported := set.New[int]()
	for _, f := range findings {
		reported.Add(f.Line)
	}
	lines := strings.SplitAfter(message, "\n")
	var out strings.Builder
	for n, line := range lines {
		if !reported.Contains(n + 1) {
			out.WriteString(line)
		}
	}
	return out.String()
}

// messageSentences answers the byte span of each sentence. A sentence ends at
// a period, an exclamation mark, a question mark or a line break. A period
// between letters or digits sits inside a name such as CLAUDE.md.
func messageSentences(message string) [][2]int {
	var out [][2]int
	start := 0
	for i := 0; i < len(message); i++ {
		if !endsMessageSentence(message, i) {
			continue
		}
		out = append(out, [2]int{start, i + 1})
		start = i + 1
	}
	if start < len(message) {
		out = append(out, [2]int{start, len(message)})
	}
	return out
}

func endsMessageSentence(text string, at int) bool {
	switch text[at] {
	case '\n', '!', '?':
		return true
	case '.':
		return at == 0 || at+1 >= len(text) || !isAlnumByte(text[at-1]) || !isAlnumByte(text[at+1])
	}
	return false
}

func isAlnumByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// lineAtOffset answers the line, counting from one, that holds byte at.
func lineAtOffset(text string, at int) int {
	return strings.Count(text[:at], "\n") + 1
}

// messageLine answers the line number, counting from one, of the first line in
// text that equals line. A line it cannot find answers one.
func messageLine(text, line string) int {
	for i, candidate := range strings.Split(text, "\n") {
		if strings.TrimSpace(candidate) == strings.TrimSpace(line) {
			return i + 1
		}
	}
	return 1
}
