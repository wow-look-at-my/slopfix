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
func tableRows(lines []string, starts []int, t [2]int) []row {
	var out []row
	for i := t[0] + 2; i <= t[1]; i++ {
		r := row{line: i}
		entry := true
		for _, c := range cells(lines[i]) {
			text := lines[i][c[0]:c[1]]
			if strings.TrimSpace(text) == "" {
				continue
			}
			cuts := inlineCuts(text, starts[i]+c[0])
			r.cuts = append(r.cuts, cuts...)
			rest := strings.TrimSpace(text)
			if len(cuts) > 0 {
				rest = CutProse(rest, shifted(cuts, starts[i]+c[0]+len(text)-len(strings.TrimLeft(text, " \t"))))
			}
			n := NamesOnly(rest)
			if n == 0 {
				entry = false
			}
			r.names += n
		}
		if !entry {
			r.names = 0
		}
		out = append(out, r)
	}
	return out
}

// shifted answers the spans of cuts moved back by base.
func shifted(cuts []ProseCut, base int) []Span {
	out := make([]Span, len(cuts))
	for i, c := range cuts {
		out[i] = Span{c.Start - base, c.End - base}
	}
	return out
}
