// tighten.go shortens a comment by rewriting it, before anything is cut.
//
// Cutting is the blunt instrument: it removes a whole thought. Most over-long
// comments are not over-long by a thought, they are padded by words that carry
// nothing, and reflowed they fit. So the repair tries this before cutting, and
// cuts only what tightening cannot save.
package commentlength

import (
	"strings"

	"github.com/wow-look-at-my/slopfix/english"
)

// tighten rewrites a comment run: it drops filler, applies the shorter phrasing,
// and reflows the prose to the block's own marker and width.
//
// It returns false when nothing changed, so the caller knows tightening bought
// nothing and it is time to cut.
func tighten(text []string) ([]string, bool) {
	marker, indent, ok := commentShape(text)
	if !ok {
		return text, false
	}

	// A paragraph break is structure, so reflow each paragraph on its own and
	var out []string
	changed := false
	for _, para := range paragraphs(text) {
		if para.blank {
			out = append(out, indent+marker)
			continue
		}
		body := strings.Join(para.lines, " ")
		short := english.Fix(body, english.Comment)
		if short != body {
			changed = true
		}
		out = append(out, reflow(short, indent, marker, wrapWidth)...)
	}
	if !changed && len(out) >= len(text) {
		return text, false
	}
	return out, true
}

// Tighten rewrites a comment block shorter, reflowing its prose onto the
// block's own marker and width. It reports false when nothing it does makes
// the block smaller, so a caller knows tightening bought nothing.
func Tighten(text []string) ([]string, bool) { return tighten(text) }

// wrapWidth is the column a reflowed comment wraps at, marker included.
const wrapWidth = 78
