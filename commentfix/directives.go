// directives.go separates a block's tool lines from its prose, and cuts the
// prose to a line budget without touching them.
package commentfix

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
