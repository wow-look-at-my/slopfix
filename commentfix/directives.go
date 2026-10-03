// directives.go separates a block's tool lines from its prose, and cuts the
// prose to a line budget without touching them.
package commentfix

import "strings"

// capLines cuts a block's trailing thoughts until its prose fits in maxLines.
// The prose is reflowed first, and the directive lines stay where they are.
func capLines(b block, maxLines int) []string {
	lead, body, trail := splitDirectives(b.text)
	if tightened, _, did := tighten(body); did {
		body = tightened
	}
	for len(body) > maxLines {
		next, ok := cutLastThought(body)
		if !ok {
			break
		}
		body = next
	}
	out := make([]string, 0, len(lead)+len(body)+len(trail))
	out = append(out, lead...)
	out = append(out, body...)
	return append(out, trail...)
}

// CapLines cuts a comment run from its end until no more than maxLines of its
// lines are prose, the way the length repair cuts. With no sentence left to cut
// it drops whole lines, and closes the last sentence it keeps.
func CapLines(text []string, maxLines int) []string {
	out := capLines(block{text: text}, maxLines)
	lead, body, trail := splitDirectives(out)
	if len(body) <= maxLines {
		return out
	}
	body = append([]string{}, body[:maxLines]...)
	for len(body) > 0 && isBlankComment(body[len(body)-1]) {
		body = body[:len(body)-1]
	}
	for len(body) > 0 {
		last := body[len(body)-1]
		if closed, ok := closeLine(last); ok {
			body[len(body)-1] = closed
			break
		}
		body = body[:len(body)-1]
	}
	return append(append(append([]string{}, lead...), body...), trail...)
}

// closeLine ends a comment line as a sentence. It drops the words that open
// what the cut took away, and reports false when no word is left.
func closeLine(line string) (string, bool) {
	if endsSentence(line) {
		return line, true
	}
	words := strings.Fields(stripMarker(line))
	for len(words) > 0 && dangling.Contains(strings.ToLower(trimWord(words[len(words)-1]))) {
		words = words[:len(words)-1]
	}
	if len(words) == 0 {
		return line, false
	}
	last := words[len(words)-1]
	cut := strings.LastIndex(line, last) + len(last)
	return strings.TrimRight(line[:cut], ",;:-—") + ".", true
}

// splitDirectives separates a block's tool lines from its prose. A directive
// binds to the declaration by position -- a build constraint leads.
func splitDirectives(text []string) (lead, body, trail []string) {
	seen := false
	for _, line := range text {
		switch {
		case !isDirectiveLine(line):
			seen = true
			body = append(body, line)
		case seen:
			trail = append(trail, line)
		default:
			lead = append(lead, line)
		}
	}
	return lead, body, trail
}

// directivesOf keeps only the directive lines of a block, which is the half
// prose drops.
func directivesOf(text []string) []string {
	kept := make([]string, 0, len(text))
	for _, line := range text {
		if isDirectiveLine(line) {
			kept = append(kept, line)
		}
	}
	return kept
}

// prose drops the directive lines from a block. A build constraint is an
// instruction to a tool, so measuring it reports an essay nobody wrote.
func prose(text []string) []string {
	kept := make([]string, 0, len(text))
	for _, line := range text {
		if isDirectiveLine(line) {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}
