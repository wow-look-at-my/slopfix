// fit.go measures a repair against the budget of the code under a comment.
package commentfix

import "strings"

// cutToFit cuts the last thought from text until it fits the budget of b. It
// reports false when no cut that lands on a sentence end fits.
func cutToFit(text []string, b block) ([]string, bool) {
	for {
		if fitsCode(text, b) {
			return text, true
		}
		if wider, did := widen(text, max(floorChars, b.codeChars)); did && fitsCode(wider, b) {
			return wider, true
		}
		next, ok := cutLastThought(text)
		if !ok {
			return text, false
		}
		text = next
	}
}

// oneLine is a reflow width no comment reaches, so the prose stays on one line.
const oneLine = 1 << 20

// fitReflow lays text out at the narrowest width that fits the budget of b.
// The budget counts characters, not columns, so a single line is the last try.
func fitReflow(text, indent, marker string, b block) ([]string, bool) {
	budget := max(floorChars, b.codeChars)
	var out []string
	for _, width := range []int{min(budget, wrapWidth), budget, oneLine} {
		out = reflow(text, indent, marker, width)
		if fitsCode(out, b) {
			return out, true
		}
	}
	return out, false
}

// preferred answers the earliest cut that keeps at least a third of the words
// the longest cut keeps. A cut that reads well and keeps almost nothing loses.
func preferred(cuts [][]string) ([]string, bool) {
	most := 0
	for _, cut := range cuts {
		most = max(most, wordCount(cut))
	}
	for _, cut := range cuts {
		if 3*wordCount(cut) >= most {
			return cut, true
		}
	}
	return nil, false
}

// wordCount counts the words of a block's prose.
func wordCount(text []string) int {
	n := 0
	for _, line := range prose(text) {
		n += len(strings.Fields(stripMarker(line)))
	}
	return n
}

// fitsCode reports whether text fits the budget of the code under b.
func fitsCode(text []string, b block) bool {
	_, over := judge(block{text: text, codeLines: b.codeLines, codeChars: b.codeChars})
	return !over
}
