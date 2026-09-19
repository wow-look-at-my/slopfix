// residual.go closes the repair: whatever the table and the sentence cut left
// behind is deleted where it sits.
//
// The table rewrites what a swap can say in words. The sentence cut takes the
// rest. Neither reaches a number the extractor finds on a line no paragraph
// covers, such as an indented example inside a block comment. This pass reads
// the finding's own position and deletes those bytes. The result is a comment
// the rule reports nothing about, which is what the caller asked for.
package commentfix

import (
	"sort"
	"strings"
)

// residualPasses guards the loop: a pass deletes bytes, so the text shrinks.
const residualPasses = 8

// clearResidual deletes every number Check still finds, and reports what it
// took. Check reads comments alone, so a deletion never reaches code.
func clearResidual(filename, src string) (string, []string) {
	var removed []string
	for range residualPasses {
		hits := Check(filename, src)
		if len(hits) == 0 {
			return src, removed
		}
		var cut []string
		src, cut = deleteHits(src, hits)
		removed = append(removed, cut...)
		if len(cut) == 0 {
			// Nothing was deletable, so another pass finds the same text.
			return src, removed
		}
	}
	return src, removed
}

// deleteHits removes each hit's bytes from the line it sits on. It works back
// to front within a line, so an earlier hit's column stays valid.
func deleteHits(src string, hits []Hit) (string, []string) {
	lines := strings.Split(src, "\n")
	byLine := map[int][]Hit{}
	for _, hit := range hits {
		byLine[hit.Line] = append(byLine[hit.Line], hit)
	}
	var removed []string
	for no, onLine := range byLine {
		i := no - 1
		if i < 0 || i >= len(lines) {
			continue
		}
		sort.SliceStable(onLine, func(a, b int) bool { return onLine[a].Col > onLine[b].Col })
		line := lines[i]
		for _, hit := range onLine {
			at := hit.Col - 1
			end := at + len(hit.Number)
			if at < 0 || end > len(line) || line[at:end] != hit.Number {
				continue
			}
			line = closeGap(line[:at], line[end:])
			removed = append(removed, hit.Number)
		}
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n"), removed
}

// closeGap joins the sides of a deletion, leaving the single space a
// reader expects between words and none at all before punctuation.
func closeGap(before, after string) string {
	trimmed := strings.TrimLeft(after, " \t")
	switch {
	case trimmed == "":
		return strings.TrimRight(before, " \t")
	case strings.ContainsRune(".,;:)!?", rune(trimmed[0])):
		return strings.TrimRight(before, " \t") + trimmed
	case before == "" || strings.HasSuffix(before, " ") || strings.HasSuffix(before, "\t"):
		return before + trimmed
	}
	return before + " " + trimmed
}
