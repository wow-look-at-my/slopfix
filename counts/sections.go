package counts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/wow-look-at-my/slopfix/edit"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/markdown"
	"github.com/wow-look-at-my/slopfix/ste"
)

// IDSection names the rule over a section cited by its number.
const IDSection = "counts/section-number"

// A section number goes stale the moment a section is inserted above it. The
// repair cites the section by a slug of its title and links to its heading,
// so the citation names the section and keeps naming it.

var (
	headingLine = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)
	// numberedTitle is a heading that opens with its number: "9. Triggers", "3.2 The cap", "§4 Grants".
	numberedTitle = regexp.MustCompile(`^§?(\d+(?:\.\d+)*[a-z]?)\.?\s+(.+)$`)
	sectionRef    = regexp.MustCompile(`§(\d+(?:\.\d+)*[a-z]?)`)
	// fileBefore is a markdown file named right before a citation: "`core.md` §4" or "[x](core.md) §4".
	fileBefore = regexp.MustCompile("(?:`([^`]+\\.md)`|\\]\\(([^)#]+\\.md)\\))\\s*$")
	// fileAfter is a file named right after one: "§0 of `02-contracts.md`".
	fileAfter = regexp.MustCompile("^\\s+of\\s+(?:`([^`]+\\.md)`|\\[[^\\]]*\\]\\(([^)#]+\\.md)\\))")
)

// section is a numbered heading: the slug a citation shows and the anchor it links.
type section struct {
	slug, anchor string
}

// Sections maps each heading number in a document to its section.
func Sections(content string) map[string]section {
	out := map[string]section{}
	anchors := map[string]int{}
	slugs := map[string]int{}
	for _, block := range markdown.Split(content) {
		if block.Kind != markdown.Verbatim || len(block.Lines) != 1 {
			continue
		}
		m := headingLine.FindStringSubmatch(block.Lines[0])
		if m == nil {
			continue
		}
		anchor := uniqueAnchor(githubAnchor(m[1]), anchors)
		n := numberedTitle.FindStringSubmatch(m[1])
		if n == nil {
			continue
		}
		if _, seen := out[n[1]]; seen {
			continue
		}
		out[n[1]] = section{slug: uniqueSlug(githubAnchor(n[2]), slugs), anchor: anchor}
	}
	return out
}

// uniqueAnchor numbers a repeated heading id as GitHub does: x, x-1, x-2.
func uniqueAnchor(name string, used map[string]int) string {
	n := used[name]
	used[name] = n + 1
	if n == 0 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, n)
}

// uniqueSlug numbers a repeated title slug from the second use: x, x-2, x-3.
func uniqueSlug(name string, used map[string]int) string {
	n := used[name]
	used[name] = n + 1
	if n == 0 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, n+1)
}

// githubAnchor is the id GitHub gives a heading: lowercase, punctuation out,
// each space a hyphen.
func githubAnchor(title string) string {
	title = strings.NewReplacer("`", "", "*", "", "_", " ").Replace(title)
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// SectionHit is a section cited by number, with the link that replaces it.
type SectionHit struct {
	LineNo, Start, End int
	Number, Link       string
}

// CheckSections finds each section citation a heading can answer. path names
// the document, so a citation of another file resolves beside it. A citation
// nothing answers is left alone: it has no slug to take.
func CheckSections(path, content string) []SectionHit {
	own := Sections(content)
	others := map[string]map[string]section{}
	lookup := func(file string) map[string]section {
		if file == "" {
			return own
		}
		if s, ok := others[file]; ok {
			return s
		}
		var s map[string]section
		if path != "" {
			if body, err := os.ReadFile(filepath.Join(filepath.Dir(path), filepath.FromSlash(file))); err == nil {
				s = Sections(string(body))
			}
		}
		others[file] = s
		return s
	}
	var hits []SectionHit
	for _, line := range proseLines(content) {
		masked := blankInlineCode(line.text)
		for _, loc := range sectionRef.FindAllStringSubmatchIndex(masked, -1) {
			start, end := loc[0], loc[1]
			if start > 0 && line.text[start-1] == '[' {
				continue
			}
			number := masked[loc[2]:loc[3]]
			file := ""
			if m := fileBefore.FindStringSubmatch(line.text[:start]); m != nil {
				file = m[1] + m[2]
			} else if m := fileAfter.FindStringSubmatch(line.text[end:]); m != nil {
				file = m[1] + m[2]
			}
			sec, ok := lookup(file)[number]
			if !ok {
				continue
			}
			hits = append(hits, SectionHit{
				LineNo: line.no,
				Start:  line.offset + start,
				End:    line.offset + end,
				Number: "§" + number,
				Link:   "[§" + sec.slug + "](" + file + "#" + sec.anchor + ")",
			})
		}
	}
	return hits
}

// SectionFindings reports each citation CheckSections finds.
func SectionFindings(path, content string) []ste.Finding {
	var out []ste.Finding
	for _, hit := range CheckSections(path, content) {
		out = append(out, ste.Finding{
			Line:   hit.LineNo,
			ID:     IDSection,
			Rule:   "a section cited by its number goes stale when a section is inserted above it",
			Detail: hit.Number,
			Fix:    "Cite it by the slug of its title, as a link: " + hit.Link + ".",
		})
	}
	return out
}

func init() {
	fixer.Register(fixer.Spec{
		Label: IDSection, Families: []string{"counts"}, Rules: []string{IDSection},
		Files: []fixer.Kind{fixer.Document}, Place: 21,
		Repair: func(f *fixer.File) {
			var edits []edit.Edit
			for _, hit := range CheckSections(f.Path, f.Text()) {
				edits = append(edits, edit.Edit{Start: hit.Start, End: hit.End, Text: hit.Link})
			}
			f.Apply(edits)
		},
	})
}
