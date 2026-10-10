package commentfix

import "strings"

// lineMarkers open the prose of a single comment line. A line inside a /* */ run opens with its star.
var lineMarkers = append(append([]string{}, docMarkers...), "*")

// RewriteLineProse applies rewrite to the prose of one comment line, keeping
// its indent, its marker and the blank after the marker. A line the rewrite
// empties answers no lines, so the caller deletes it. It answers false for a
// line with no comment marker, and for a line the rewrite leaves as it was.
func RewriteLineProse(line string, rewrite func(string) (string, int)) ([]string, int, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	marker := ""
	for _, m := range lineMarkers {
		if strings.HasPrefix(trimmed, m) {
			marker = m
			break
		}
	}
	if marker == "" {
		return nil, 0, false
	}
	head := len(line) - len(trimmed) + len(marker)
	rest := line[head:]
	body := strings.TrimLeft(rest, " \t")
	head += len(rest) - len(body)
	short, took := rewrite(body)
	if took == 0 || short == body {
		return nil, 0, false
	}
	if !hasWord(short) {
		return nil, took, true
	}
	return []string{line[:head] + short}, took, true
}
