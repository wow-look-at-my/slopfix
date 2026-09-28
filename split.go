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

// section is a level-2 heading and the lines under it, up to the next level-2
// heading.
type section struct {
	title      string
	start, end int
}

// sections finds each level-2 section. A heading inside a code fence is data.
func sections(lines []string) []section {
	var out []section
	fenced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced || !strings.HasPrefix(line, "## ") {
			continue
		}
		if len(out) > 0 {
			out[len(out)-1].end = i
		}
		out = append(out, section{title: strings.TrimSpace(line[3:]), start: i, end: len(lines)})
	}
	return out
}

// Split moves the largest level-2 sections of the root file rel into DocsDir,
// until the file is at or under SplitTarget. Each section keeps its heading and
// gets a link to its new file. The text moves word for word, and each heading
// under it rises one level. Split returns the paths it wrote, relative to root.
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
	size := len([]rune(string(raw)))

	secs := sections(lines)
	sort.SliceStable(secs, func(a, b int) bool { return runes(lines[secs[a].start:secs[a].end]) > runes(lines[secs[b].start:secs[b].end]) })

	taken := map[string]bool{}
	type move struct {
		sec        section
		name, link string
	}
	var moves []move
	for _, sec := range secs {
		if size <= SplitTarget {
			break
		}
		name, err := freeDocName(root, sec.title, taken)
		if err != nil {
			return nil, err
		}
		link := fmt.Sprintf("[%s](%s) holds this section.", name, name)
		size -= runes(lines[sec.start+1:sec.end]) - runes([]string{"", link, ""})
		moves = append(moves, move{sec, name, link})
	}
	if len(moves) == 0 {
		return nil, nil
	}

	// A later section first, so each cut leaves the earlier line numbers valid.
	sort.Slice(moves, func(a, b int) bool { return moves[a].sec.start > moves[b].sec.start })
	var written []string
	for _, m := range moves {
		name := m.name
		body := strings.Trim(strings.Join(raise(lines[m.sec.start+1:m.sec.end]), "\n"), "\n")
		doc := "# " + m.sec.title + "\n\n" + body + "\n"
		if !dryRun {
			if err := os.MkdirAll(filepath.Join(root, DocsDir), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(doc), info.Mode().Perm()); err != nil {
				return nil, err
			}
		}
		written = append(written, name)
		tail := append([]string{"", m.link, ""}, lines[m.sec.end:]...)
		if m.sec.end == len(lines) {
			tail = []string{"", m.link, ""}
		}
		lines = append(lines[:m.sec.start+1], tail...)
	}
	sort.Strings(written)
	if dryRun {
		return written, nil
	}
	out := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	return written, os.WriteFile(file, []byte(out), info.Mode().Perm())
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
