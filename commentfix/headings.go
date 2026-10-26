// headings.go keeps a comment's headings through the length repair.
package commentfix

import "strings"

// trim cuts the block's prose until it fits, and keeps each heading that opens it.
// A heading is structure, so the budget does not weigh it.
func trim(b block) []string {
	return keepHeadings(b.text, cutProse(b))
}

// keepHeadings puts back each heading that opens text when out lost it. A cut
// that kept the sentence under a heading keeps the heading too.
func keepHeadings(text, out []string) []string {
	var lead []string
	for _, line := range text {
		if isBlankComment(line) {
			continue
		}
		if !headingLine(line) {
			break
		}
		lead = append(lead, line)
	}
	if len(lead) == 0 || len(out) == 0 || headingLine(out[0]) {
		return out
	}
	return append(lead, out...)
}

// headingLine reports a comment line whose prose is a markdown heading, in a
// line comment or inside a /* */ comment.
func headingLine(line string) bool {
	t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "*/"))
	for _, opener := range []string{"/**", "/*", "*"} {
		if rest, ok := strings.CutPrefix(t, opener); ok {
			return isHeading(strings.TrimSpace(rest))
		}
	}
	return isHeading(stripMarker(t))
}

// weighed answers the lines of a block the budget weighs. A directive, a
// heading and the bare marker line gofmt puts before a directive are not prose.
func weighed(text []string) []string {
	directives := directiveMask(text)
	kept := make([]string, 0, len(text))
	for i, line := range text {
		if directives[i] || headingLine(line) {
			continue
		}
		if i+1 < len(text) && directives[i+1] && isBlankComment(line) {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

// fittedOpening answers the opening sentence of body that fits the budget of b,
// divided where it runs past the STE cap. It reports false when none fits.
func fittedOpening(body []string, b block) ([]string, bool) {
	fit := func(sentence, indent, marker string) ([]string, bool) {
		for _, width := range []int{wrapWidth, oneLine} {
			if out := reflow(sentence, indent, marker, width); fitsCode(out, b) {
				return out, true
			}
		}
		return nil, false
	}
	opening, ok := steOpening(body, fit, true)
	if !ok || len(opening) == 0 || !fitsCode(opening, b) {
		return nil, false
	}
	return keepHeadings(body, opening), true
}
