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
	closer := ""
	if len(body) > 0 {
		closer = body[len(body)-1]
	}
	// A closer on its own line comes back after a cut, so it takes a line of the cap.
	limit := maxLines
	if strings.TrimSpace(closer) == "*/" {
		limit--
	}
	for len(body) > limit {
		next, ok := cutLastThought(body)
		if !ok {
			break
		}
		body = next
	}
	if len(body) > limit {
		body = dropToSentenceEnd(body, limit)
	}
	body = reclosed(body, closer)
	out := make([]string, 0, len(lead)+len(body)+len(trail))
	out = append(out, lead...)
	out = append(out, body...)
	return append(out, trail...)
}

// dropToSentenceEnd drops whole lines from the end of body until no more than
// maxLines are left and the last line ends a sentence. With no such line, body
// stays as it is, for a rewrite by hand.
func dropToSentenceEnd(body []string, maxLines int) []string {
	for n := min(maxLines, len(body)); n > 0; n-- {
		last := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(stripMarker(body[n-1])), "*/"))
		if isBlankComment(body[n-1]) || !endsSentence(last) {
			continue
		}
		return body[:n]
	}
	return body
}

// reclosed puts back the */ a cut took with the last thought, so the code
// under the block does not read as comment. closer is the body's last line
// before the cut.
func reclosed(body []string, closer string) []string {
	end := strings.TrimSpace(closer)
	if !strings.HasSuffix(end, "*/") || len(body) == 0 || strings.HasSuffix(strings.TrimSpace(body[len(body)-1]), "*/") {
		return body
	}
	body = append([]string{}, body...)
	for len(body) > 1 && isBlankComment(body[len(body)-1]) {
		body = body[:len(body)-1]
	}
	if end == "*/" {
		return append(body, closer)
	}
	body[len(body)-1] += " */"
	return body
}

// CapLines cuts a comment run from its end until no more than maxLines of its
// lines are prose, the way the length repair cuts.
func CapLines(text []string, maxLines int) []string {
	return capLines(block{text: text}, maxLines)
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
