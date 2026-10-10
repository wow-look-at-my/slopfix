package commentfix

import "strings"

// rowMarkers open the prose of a single comment row. A row inside a /* */ run opens with its star.
var rowMarkers = append(append([]string{}, docMarkers...), "*")

// CutRow applies cut to the prose of one comment row, keeping its indent, its
// marker and the blank after the marker. A row the cut empties answers no
// lines, so the caller deletes it. It answers false for a row with no comment
// marker, and for a row the cut leaves as it was.
func CutRow(line string, cut func(string) (string, int)) ([]string, int, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	marker := ""
	for _, m := range rowMarkers {
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
	short, took := cut(body)
	if took == 0 || short == body {
		return nil, 0, false
	}
	if !hasWord(short) {
		return nil, took, true
	}
	return []string{line[:head] + short}, took, true
}
