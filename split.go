package slopfix

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// SplitTarget is the size a split brings a root file down to. The gap under CharBudget leaves room for the next edit.
const SplitTarget = CharBudget * 4 / 5

// DocsDir holds the sections a split moves out of a root file.
const DocsDir = "docs"

// section is a heading and the lines under it, up to the next heading of the
// same level or higher.
type section struct {
	title      string
	start, end int
}

// headingLevel answers how many marks open a heading line. A line that is no
// heading answers nothing.
func headingLevel(line string) int {
	level := len(line) - len(strings.TrimLeft(line, "#"))
	if level < 1 || level > 6 || len(line) == level || line[level] != ' ' {
		return 0
	}
	return level
}

// sections finds each section at a heading level. A heading inside a code
// fence is data.
func sections(lines []string, level int) []section {
	var out []section
	fenced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		at := headingLevel(line)
		if fenced || at == 0 || at > level {
			continue
		}
		if len(out) > 0 && out[len(out)-1].end == len(lines) {
			out[len(out)-1].end = i
		}
		if at == level {
			out = append(out, section{title: strings.TrimSpace(line[level:]), start: i, end: len(lines)})
		}
	}
	return out
}

// splitLevels are the heading levels a split moves sections at, in order.
var splitLevels = []int{2, 3, 1, 4, 5, 6}

// Split moves the largest sections of the root file rel into DocsDir, until the
// file is at or under SplitTarget. It takes level-2 sections first, then the
// other levels. Each section keeps its heading and gets a link to its new file.
// The text moves word for word, and under a level-2 heading each heading rises
// one level. A file with no heading left to move loses its tail to a file of
// its own. Split returns the paths it wrote, relative to root.
func Split(root, rel string, dryRun bool) ([]string, error) {
	file := filepath.Join(root, rel)
	info, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	s := splitter{root: root, dryRun: dryRun, perm: info.Mode().Perm(), taken: map[string]bool{}}
	for _, level := range splitLevels {
		if runes(lines) <= SplitTarget {
			break
		}
		if lines, err = s.moveSections(lines, level); err != nil {
			return nil, err
		}
	}
	if runes(lines) > SplitTarget {
		if lines, err = s.moveTail(lines, strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))); err != nil {
			return nil, err
		}
	}
	if len(s.written) == 0 {
		return nil, nil
	}
	sort.Strings(s.written)
	if dryRun {
		return s.written, nil
	}
	out := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	return s.written, os.WriteFile(file, []byte(out), info.Mode().Perm())
}

// splitter writes the files a split moves text into.
type splitter struct {
	root    string
	dryRun  bool
	perm    os.FileMode
	taken   map[string]bool
	written []string
}

// write puts a moved document into DocsDir.
func (s *splitter) write(name, doc string) error {
	s.written = append(s.written, name)
	if s.dryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(s.root, DocsDir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.root, filepath.FromSlash(name)), []byte(doc), s.perm)
}

// moveSections moves the largest sections at level until the text is at or
// under SplitTarget, and answers what stays.
func (s *splitter) moveSections(lines []string, level int) ([]string, error) {
	secs := sections(lines, level)
	sort.SliceStable(secs, func(a, b int) bool {
		return runes(lines[secs[a].start:secs[a].end]) > runes(lines[secs[b].start:secs[b].end])
	})
	type move struct {
		sec        section
		name, link string
	}
	size := runes(lines)
	var moves []move
	for _, sec := range secs {
		if size <= SplitTarget {
			break
		}
		// A section shorter than the link that replaces it saves nothing.
		if runes(lines[sec.start+1:sec.end]) <= runes([]string{"", "[docs/x.md](docs/x.md) holds this section.", ""}) {
			continue
		}
		name, err := freeDocName(s.root, sec.title, s.taken)
		if err != nil {
			return nil, err
		}
		link := fmt.Sprintf("[%s](%s) holds this section.", name, name)
		size -= runes(lines[sec.start+1:sec.end]) - runes([]string{"", link, ""})
		moves = append(moves, move{sec, name, link})
	}
	// A later section first, so each cut leaves the earlier line numbers valid.
	sort.Slice(moves, func(a, b int) bool { return moves[a].sec.start > moves[b].sec.start })
	for _, m := range moves {
		body := lines[m.sec.start+1 : m.sec.end]
		if level == 2 {
			body = raise(body)
		}
		doc := "# " + m.sec.title + "\n\n" + strings.Trim(strings.Join(body, "\n"), "\n") + "\n"
		if err := s.write(m.name, doc); err != nil {
			return nil, err
		}
		tail := append([]string{"", m.link, ""}, lines[m.sec.end:]...)
		lines = append(lines[:m.sec.start+1:m.sec.start+1], tail...)
	}
	return lines, nil
}

// moveTail moves the lines past the budget into a file of their own, and
// leaves a link where they stood. The cut lands on a blank line outside a
// fence where one fits. A cut inside a fence closes it, and the moved part
// opens it again.
func (s *splitter) moveTail(lines []string, stem string) ([]string, error) {
	name, err := freeDocName(s.root, stem+" continued", s.taken)
	if err != nil {
		return nil, err
	}
	link := fmt.Sprintf("[%s](%s) holds the rest of this file.", name, name)
	limit := SplitTarget - runes([]string{"", link, "```"}) - 1
	cut, blankCut := -1, -1
	fence, fenceAtCut := "", ""
	size, sizeAtBlank := 0, 0
	for i, line := range lines {
		if size+len([]rune(line))+1 > limit {
			break
		}
		size += len([]rune(line)) + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			if fence == "" {
				fence = trimmed
			} else {
				fence = ""
			}
		}
		cut, fenceAtCut = i+1, fence
		if trimmed == "" && fence == "" {
			blankCut, sizeAtBlank = i+1, size
		}
	}
	// A blank line is the better cut, unless it keeps less than half of what fits.
	if blankCut > 0 && 2*sizeAtBlank >= limit {
		cut, fenceAtCut = blankCut, ""
	}
	if cut <= 0 {
		// A single line is longer than the budget, so it divides at a blank.
		head, rest := cutLine(lines[0], limit)
		lines = append([]string{head, rest}, lines[1:]...)
		cut, fenceAtCut = 1, ""
	}
	moved := append([]string{}, lines[cut:]...)
	kept := append([]string{}, lines[:cut]...)
	if fenceAtCut != "" {
		kept = append(kept, fenceMarker(fenceAtCut))
		moved = append([]string{fenceAtCut}, moved...)
	}
	doc := "# " + stem + ", continued\n\n" + strings.Trim(strings.Join(moved, "\n"), "\n") + "\n"
	if err := s.write(name, doc); err != nil {
		return nil, err
	}
	return append(kept, "", link), nil
}

// fenceMarker answers the bare marker that closes a fence an opener started.
func fenceMarker(opener string) string {
	marker := opener[:1]
	return strings.Repeat(marker, len(opener)-len(strings.TrimLeft(opener, marker)))
}

// cutLine divides a line at the last blank before limit characters, or at the
// limit itself when the line holds no blank there.
func cutLine(line string, limit int) (string, string) {
	r := []rune(line)
	at := min(max(limit, 1), len(r))
	for i := at - 1; i > 0; i-- {
		if r[i] == ' ' {
			return string(r[:i]), string(r[i+1:])
		}
	}
	return string(r[:at]), string(r[at:])
}

func raise(lines []string) []string {
	out := make([]string, len(lines))
	fenced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(line, "###") {
			line = line[1:]
		}
		out[i] = line
	}
	return out
}

// freeDocName names a new file in DocsDir after a heading. It never names a file
// that exists, or one this split already took.
func freeDocName(root, title string, taken map[string]bool) (string, error) {
	base := slug(title)
	for n := 1; ; n++ {
		stem := base
		if n > 1 {
			stem = fmt.Sprintf("%s-%d", base, n)
		}
		name := path.Join(DocsDir, stem+".md")
		if taken[name] {
			continue
		}
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
		if os.IsNotExist(err) {
			taken[name] = true
			return name, nil
		}
		if err != nil {
			return "", err
		}
	}
}

// slug turns a heading into a file stem: lower case letters and digits, with a
// hyphen for each run of anything else.
func slug(title string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			hyphen = false
		} else if !hyphen && b.Len() > 0 {
			b.WriteByte('-')
			hyphen = true
		}
	}
	if s := strings.TrimRight(b.String(), "-"); s != "" {
		return s
	}
	return "section"
}

// runes counts the characters of lines joined back into text.
func runes(lines []string) int {
	return len([]rune(strings.Join(lines, "\n")))
}
