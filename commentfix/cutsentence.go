package commentfix

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// CutSentence takes every sentence that holds name out of a comment block of
// line comments, and reflows each paragraph it changed. A sentence can run
// over several lines. The cut takes that sentence and nothing else. A
// paragraph the cut empties is gone. It reports false when the block has
// another shape, or no prose sentence holds name.
func CutSentence(text []string, name string) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, false
	}
	var out []string
	changed := false
	for _, para := range paragraphs(text) {
		switch {
		case para.blank:
			// A break goes only between paragraphs that stay.
			if len(out) > 0 && out[len(out)-1] != indent+marker {
				out = append(out, indent+marker)
			}
			continue
		case para.verbatim || para.list():
			out = append(out, para.raw...)
			continue
		}
		body := strings.Join(para.lines, " ")
		if !strings.Contains(body, name) {
			out = append(out, para.raw...)
			continue
		}
		var kept []string
		for _, s := range ste.Sentences(body) {
			if !strings.Contains(s, name) {
				kept = append(kept, strings.TrimSpace(s))
			}
		}
		short := strings.Join(kept, " ")
		if short == body {
			out = append(out, para.raw...)
			continue
		}
		changed = true
		if hasWord(short) {
			out = append(out, reflow(short, indent, marker, wrapWidth)...)
		}
	}
	for len(out) > 0 && out[len(out)-1] == indent+marker {
		out = out[:len(out)-1]
	}
	if !changed {
		return text, false
	}
	return out, true
}
