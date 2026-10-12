package commentfix

import (
	"regexp"
	"strings"

	"github.com/wow-look-at-my/slopfix/ste"
)

// CutSentences drops each sentence of a comment run that names name, and
// reflows the paragraphs it cut onto the run's marker and width. A paragraph
// it does not cut keeps its lines as written. It answers the sentences it
// dropped, and false when no sentence of the run names name.
func CutSentences(text []string, name string) ([]string, []string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, nil, false
	}
	names := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(name) + `([^A-Za-z0-9_]|$)`)
	var out, dropped []string
	for _, para := range paragraphs(text) {
		if para.blank {
			// A paragraph the cut emptied leaves no second blank line behind it.
			if len(out) > 0 && out[len(out)-1] != indent+marker {
				out = append(out, indent+marker)
			}
			continue
		}
		if para.verbatim || para.list() {
			out = append(out, para.raw...)
			continue
		}
		var kept []string
		for _, sentence := range ste.Sentences(strings.Join(para.lines, " ")) {
			if names.MatchString(sentence) {
				dropped = append(dropped, strings.TrimSpace(sentence))
				continue
			}
			kept = append(kept, strings.TrimSpace(sentence))
		}
		if len(kept) == len(ste.Sentences(strings.Join(para.lines, " "))) {
			out = append(out, para.raw...)
			continue
		}
		if body := strings.Join(kept, " "); hasWord(body) {
			out = append(out, reflow(body, indent, marker, wrapWidth)...)
		}
	}
	for len(out) > 0 && out[len(out)-1] == indent+marker {
		out = out[:len(out)-1]
	}
	return out, dropped, len(dropped) > 0
}
