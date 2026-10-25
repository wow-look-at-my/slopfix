// stacked.go repairs a run of several /* */ comments on adjoining lines, such
// as a doc comment left above another doc comment.

package commentfix

import "strings"

// stackedComment is one /* */ comment of a run, its prose kept as line
// comments so the sentence cuts read it.
type stackedComment struct {
	shape blockShape
	lines []string
}

// splitStacked answers each /* */ comment of a run, and false when a line of
// the run belongs to none or the run holds a single comment.
func splitStacked(text []string) ([]stackedComment, bool) {
	var out []stackedComment
	start := -1
	for i, line := range text {
		t := strings.TrimSpace(line)
		if start < 0 {
			if !strings.HasPrefix(t, "/*") {
				return nil, false
			}
			start = i
		}
		if !strings.HasSuffix(t, "*/") {
			continue
		}
		shape, body, ok := readBlock(text[start : i+1])
		if !ok {
			return nil, false
		}
		out = append(out, stackedComment{shape: shape, lines: asLines(shape.indent, body)})
		start = -1
	}
	return out, start < 0 && len(out) > 1
}

// renderStacked writes every comment of a run back in its own shape.
func renderStacked(comments []stackedComment) []string {
	var out []string
	for _, c := range comments {
		out = append(out, c.shape.render(fromLines(c.lines))...)
	}
	return out
}

// repairStacked cuts a run of /* */ comments from its end until it fits the
// code under b. The last comment loses its trailing thoughts first, down to
// its opening sentence on one line, and then the comment before it does. Each
// comment keeps its own opener and closer. A run no cut fits keeps its text,
// as a single comment no cut fits does.
func repairStacked(b block) ([]string, bool) {
	comments, ok := splitStacked(b.text)
	if !ok {
		return nil, false
	}
	for g := len(comments) - 1; g >= 0; g-- {
		for {
			if out := renderStacked(comments); fitsCode(out, b) {
				return out, true
			}
			if wider, did := widen(comments[g].lines, oneLine); did {
				trial := append([]stackedComment{}, comments...)
				trial[g].lines = wider
				if out := renderStacked(trial); fitsCode(out, b) {
					return out, true
				}
			}
			next, cut := cutLastThought(comments[g].lines)
			if !cut {
				break
			}
			comments[g].lines = next
		}
		if wider, did := widen(comments[g].lines, oneLine); did {
			comments[g].lines = wider
		}
	}
	return b.text, true
}
