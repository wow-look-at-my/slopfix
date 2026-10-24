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
	// A thought with no stop gets one, so no rewrite and no cut welds it to the next.
	if capCount(body) > limit {
		if divided, ok := divideProse(body); ok {
			body = divided
		}
	}
	// Rewrite the prose to its budget before any cut. A block the rewrite brings
	// under the cap keeps every thought it had.
	if capCount(body) > limit {
		if fitted, ok := fitVolume(body, limit); ok {
			body = fitted
		}
	}
	body = cutThoughts(body, limit)
	if capCount(body) > limit {
		body = dropToSentenceEnd(body, limit)
	}
	// With no sentence end to cut at, the opening sentence stays, divided where it runs past the STE cap.
	if capCount(body) > limit {
		opening, whole := steOpening(body, func(sentence, indent, marker string) ([]string, bool) {
			out := reflow(sentence, indent, marker, wrapWidth)
			return out, len(out) <= limit
		}, true)
		if headed := keepHeadings(body, opening); whole && len(headed) <= limit {
			body = headed
		} else if whole && len(opening) <= limit {
			body = opening
		}
	}
	// Every rewrite has run, so the line the cap keeps is trimmed back to its
	// last sentence end.
	if capCount(body) > limit {
		if trimmed, ok := trimBodyToSentenceEnd(body, limit); ok {
			body = trimmed
		}
	}
	body = reclosed(body, closer)
	out := make([]string, 0, len(lead)+len(body)+len(trail))
	out = append(out, lead...)
	out = append(out, body...)
	return append(out, trail...)
}

// cutThoughts cuts the last thought from body until no more than limit of its lines count.
func cutThoughts(body []string, limit int) []string {
	for capCount(body) > limit {
		next, ok := cutLastThought(body)
		if !ok {
			break
		}
		body = next
	}
	return body
}

// capCount counts the lines of a run the volume cap weighs.
func capCount(text []string) int {
	n := len(text)
	for _, data := range DataLines(text) {
		if data {
			n--
		}
	}
	return n
}

// DataLines marks each line of a comment run that a code block laid out by
// hand holds. No repair reflows such a line, and the volume cap does not count it.
func DataLines(text []string) []bool {
	out := make([]bool, len(text))
	directives := directiveMask(text)
	at := 0
	for _, para := range paragraphs(text) {
		switch {
		case para.blank:
			at++
		case para.verbatim:
			for range para.raw {
				out[at] = !directives[at] && !isHeading(stripMarker(text[at]))
				at++
			}
		default:
			at += len(para.lines)
		}
	}
	return out
}

// dropToSentenceEnd drops whole lines from the end of body until no more than
// maxLines are left and the last line ends a sentence. With no such line, body
// stays as it is, and the cuts after this one decide.
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

// trimBodyToSentenceEnd keeps the first maxLines lines of a body. It trims the
// last of them back to its last sentence end, keeping a closer that follows it.
// It reports false when that line holds no sentence end.
func trimBodyToSentenceEnd(body []string, maxLines int) ([]string, bool) {
	if maxLines <= 0 || len(body) <= maxLines {
		return nil, false
	}
	kept := append([]string{}, body[:maxLines]...)
	trimmed, ok := trimLineToSentenceEnd(kept[maxLines-1])
	if !ok {
		return nil, false
	}
	kept[maxLines-1] = trimmed
	return reclosed(kept, body[len(body)-1]), true
}

// trimLineToSentenceEnd removes whatever follows the last sentence end on a
// comment line, keeping a block closer that follows it.
func trimLineToSentenceEnd(line string) (string, bool) {
	cut := strings.LastIndexAny(line, ".!?")
	if cut < 0 {
		return "", false
	}
	trimmed := line[:cut+1]
	if rest := line[cut+1:]; strings.Contains(rest, "*/") {
		trimmed += " */"
	}
	return trimmed, true
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
	directives := directiveMask(text)
	for i, line := range text {
		switch {
		case !directives[i]:
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
	directives := directiveMask(text)
	for i, line := range text {
		if directives[i] {
			kept = append(kept, line)
		}
	}
	return kept
}

// prose drops the directive lines from a block. A build constraint is an
// instruction to a tool, so measuring it reports an essay nobody wrote.
func prose(text []string) []string {
	kept := make([]string, 0, len(text))
	directives := directiveMask(text)
	for i, line := range text {
		if directives[i] {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}
