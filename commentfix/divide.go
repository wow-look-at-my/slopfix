// divide.go divides a comment's prose into sentences, so a cut can land on a sentence end.
package commentfix

import "strings"

// divideProse divides each prose paragraph of a run of line comments into
// sentences, as ste/sentence-length divides them. A line that ends a thought
// with no stop gets one, and so does the paragraph's end. A heading, a list
// and a code block stay as written. It reports false when nothing changes.
func divideProse(text []string) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, false
	}
	var out []string
	changed := false
	for _, para := range paragraphs(text) {
		switch {
		case para.blank:
			out = append(out, indent+marker)
			continue
		case para.verbatim || para.list():
			out = append(out, para.raw...)
			continue
		}
		joined := closeThoughts(para.lines)
		if !endsSentence(joined) {
			joined += "."
		}
		divided := divideSentences(joined)
		if strings.Join(strings.Fields(divided), " ") == strings.Join(strings.Fields(strings.Join(para.lines, " ")), " ") {
			out = append(out, para.raw...)
			continue
		}
		changed = true
		width := wrapWidth
		for _, line := range para.raw {
			width = max(width, len(line))
		}
		out = append(out, reflow(divided, indent, marker, width)...)
	}
	if !changed {
		return text, false
	}
	return out, true
}
