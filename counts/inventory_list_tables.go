// inventory_list_tables.go finds the fences, headings and tables of a document.

package counts

import (
	"regexp"
	"strings"
)

var (
	fenceOpen   = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	headingOpen = regexp.MustCompile(`^ {0,3}#{1,6}(\s|$)`)
	tableRule   = regexp.MustCompile(`^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?$`)
)

// structure is where a document's fences, headings and tables sit, by line from zero.
type structure struct {
	// tables holds the header line and the last body line of each table.
	tables  [][2]int
	fence   []bool
	heading []bool
	table   []bool
}

func scan(lines []string) structure {
	s := structure{fence: make([]bool, len(lines)), heading: make([]bool, len(lines)), table: make([]bool, len(lines))}
	for i := 0; i < len(lines); i++ {
		if m := fenceOpen.FindStringSubmatch(lines[i]); m != nil {
			j := i + 1
			for j < len(lines) && !closesFence(lines[j], m[1]) {
				j++
			}
			end := min(j, len(lines)-1)
			for k := i; k <= end; k++ {
				s.fence[k] = true
			}
			i = end
			continue
		}
		if headingOpen.MatchString(lines[i]) {
			s.heading[i] = true
			continue
		}
		if piped(lines[i]) && i+1 < len(lines) && tableRule.MatchString(strings.TrimSpace(lines[i+1])) {
			j := i + 2
			for j < len(lines) && piped(lines[j]) && !fenceOpen.MatchString(lines[j]) {
				j++
			}
			s.tables = append(s.tables, [2]int{i, j - 1})
			for k := i; k < j; k++ {
				s.table[k] = true
			}
			i = j - 1
		}
	}
	return s
}

func piped(line string) bool { return strings.HasPrefix(strings.TrimSpace(line), "|") }

func closesFence(line, fence string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == ""
}

// cells answers the byte range of each cell of a table row, between its pipes.
// An escaped pipe divides no cell.
func cells(line string) [][2]int {
	var pipes []int
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case '|':
			pipes = append(pipes, i)
		}
	}
	var out [][2]int
	for k := 0; k+1 < len(pipes); k++ {
		out = append(out, [2]int{pipes[k] + 1, pipes[k+1]})
	}
	if n := len(pipes); n > 0 && strings.TrimSpace(line[pipes[n-1]+1:]) != "" {
		out = append(out, [2]int{pipes[n-1] + 1, len(line)})
	}
	return out
}

// headingText answers where the words of a heading line open.
func headingText(line string) int {
	at := len(line) - len(strings.TrimLeft(line, " "))
	for at < len(line) && line[at] == '#' {
		at++
	}
	for at < len(line) && (line[at] == ' ' || line[at] == '\t') {
		at++
	}
	return at
}

// inlineCuts answers the enumerations of text at offset base, when text keeps
// a word after they go. A cell or a heading cannot be deleted whole, so a cut
// that empties one is no repair.
func inlineCuts(text string, base int) []ProseCut {
	lead := len(text) - len(strings.TrimLeft(text, " \t"))
	text = strings.TrimSpace(text)
	cuts := ProseCuts(text)
	if len(cuts) == 0 || !keepsWords(text, spansOf(cuts)) {
		return nil
	}
	for i := range cuts {
		cuts[i].Start += base + lead
		cuts[i].End += base + lead
	}
	return cuts
}

// row is what one table body row holds.
type row struct {
	line  int
	names int
	cuts  []ProseCut
}

// tableRows judges each body row of table t. A row whose cells only name
// things has names above zero. A row that states something keeps its cuts.
//
// A cell names things when it holds only members, or when it is one word in
// a column of names. A column of names holds only such cells, and members in
// half of them or more. A cell of members in a column of descriptions is the
// description of its row, so the row stays.
func tableRows(lines []string, starts []int, t [2]int) []row {
	type cell struct {
		col, names int
		word       bool
	}
	var body [][]cell
	nameCol := map[int]bool{}
	filled, members := map[int]int{}, map[int]int{}
	for i := t[0] + 2; i <= t[1]; i++ {
		var rowCells []cell
		for k, c := range cells(lines[i]) {
			text := strings.TrimSpace(lines[i][c[0]:c[1]])
			if text == "" {
				continue
			}
			ce := cell{col: k, names: NamesOnly(text), word: len(strings.Fields(text)) == 1}
			rowCells = append(rowCells, ce)
			filled[k]++
			if ce.names > 0 {
				members[k]++
			}
			if _, seen := nameCol[k]; !seen {
				nameCol[k] = true
			}
			if ce.names == 0 && !ce.word {
				nameCol[k] = false
			}
		}
		body = append(body, rowCells)
	}
	for k := range nameCol {
		nameCol[k] = nameCol[k] && members[k]*2 >= filled[k]
	}
	var out []row
	for n, rowCells := range body {
		i := t[0] + 2 + n
		r := row{line: i}
		for _, ce := range rowCells {
			if !nameCol[ce.col] {
				r.names = 0
				break
			}
			r.names += max(ce.names, 1)
		}
		if r.names == 0 {
			for _, c := range cells(lines[i]) {
				r.cuts = append(r.cuts, inlineCuts(lines[i][c[0]:c[1]], starts[i]+c[0])...)
			}
		}
		out = append(out, r)
	}
	return out
}
